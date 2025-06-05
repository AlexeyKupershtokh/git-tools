package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

var (
	optM = flag.Bool("m", false, "Open master branch")
	optR = flag.Bool("r", false, "Open root of repo")
	optN = flag.Bool("n", false, "Do not open browser, just print the link")
)

func main() {
	flag.Parse()

	args := flag.Args()
	var filePath string
	if len(args) > 0 {
		filePath = args[0]
	}

	// Найти .git директорию
	gitDir, err := findGitDir(".")
	if err != nil {
		log.Fatal("Not inside a git repository.")
	}

	// Читаем remote.origin.url из .git/config
	configPath := filepath.Join(gitDir, "config")
	remoteURL, err := parseRemoteURL(configPath)
	if err != nil {
		log.Fatalf("Could not read origin URL from config: %v", err)
	}

	// Парсим project и repoName из SSH URL
	project, repoName, err := parseRepoInfo(remoteURL)
	if err != nil {
		log.Fatalf("Invalid remote URL: %v", err)
	}

	domain, err := parseDomain(remoteURL)
	if err != nil {
		log.Fatalf("Invalid remote URL: %v", err)
	}

	baseURL := fmt.Sprintf("https://%s/projects/%s/repos/%s/browse", domain, project, repoName)

	// Определение бранча
	var branch string
	if *optM {
		branch = "refs/heads/master"
	} else {
		headFile := filepath.Join(gitDir, "HEAD")
		branch, err = getCurrentBranch(headFile)
		if err != nil {
			log.Fatalf("Could not determine current branch: %v", err)
		}
	}

	// Определение пути
	var relativePath string
	if !*optR {
		workdir := filepath.Dir(gitDir) // корень репозитория

		if filePath != "" {
			fp := filepath.Clean(filePath)
			if _, err := os.Stat(fp); os.IsNotExist(err) {
				log.Fatalf("Path does not exist locally: %s", fp)
			}
			absFP, _ := filepath.Abs(fp)
			rel, err := filepath.Rel(workdir, absFP)
			if err != nil {
				log.Fatalf("Could not compute relative path for %s", fp)
			}
			relativePath = "/" + rel
		} else {
			wd, _ := os.Getwd()
			rel, err := filepath.Rel(workdir, wd)
			if err != nil || rel == "." {
				relativePath = ""
			} else {
				relativePath = "/" + rel
			}
		}
	}

	// Строим URL
	finalURL := baseURL + relativePath

	if branch != "refs/heads/master" {
		encodedBranch := url.QueryEscape(branch)
		finalURL += "?at=" + encodedBranch
	}

	if !*optN {
		fmt.Printf("Opening: %s\n", finalURL)
		openBrowser(finalURL)
	} else {
		fmt.Printf("URL: %s\n", finalURL)
	}
}

// Находим .git директорию
func findGitDir(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		gitDir := filepath.Join(dir, ".git")
		if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
			return gitDir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("not in a git repository")
}

// Парсим remote.origin.url из .git/config
func parseRemoteURL(configPath string) (string, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(data), "\n")
	var inOrigin bool
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[remote \"origin\"]") {
			inOrigin = true
		} else if inOrigin && strings.HasPrefix(line, "url = ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "url = ")), nil
		} else if strings.HasPrefix(line, "[") {
			inOrigin = false
		}
	}
	return "", fmt.Errorf("no origin remote found")
}

// Парсим домен и repoName из SSH URL
func parseDomain(remoteURL string) (string, error) {
	u, err := url.Parse(remoteURL)
	if err != nil {
		return "", err
	}

	host, _, err := net.SplitHostPort(u.Host)
	if err != nil {
		return "", err
	}

	return host, nil
}

// Парсим project и repoName из SSH URL
func parseRepoInfo(remoteURL string) (project, repoName string, err error) {
	r := regexp.MustCompile(`ssh://[^/]+/([^/]+)/([^/.]+)(\.git)?`)
	matches := r.FindStringSubmatch(remoteURL)
	if len(matches) < 3 {
		return "", "", fmt.Errorf("unsupported remote URL format: %s", remoteURL)
	}
	return matches[1], matches[2], nil
}

// Читаем .git/HEAD и определяем бранч
func getCurrentBranch(headPath string) (string, error) {
	data, err := os.ReadFile(headPath)
	if err != nil {
		return "", err
	}
	ref := strings.TrimSpace(string(data))
	if strings.HasPrefix(ref, "ref: ") {
		return strings.TrimPrefix(ref, "ref: "), nil
	}
	return "", fmt.Errorf("could not parse HEAD")
}

// Открытие ссылки в браузере
func openBrowser(url string) {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open browser: %v\n", err)
		fmt.Println("Please open the following URL manually:")
		fmt.Println(url)
	}
}
