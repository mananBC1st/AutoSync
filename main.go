package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"
)

type BlogConfig struct {
	ProjectDir    string `json:"projectDir"`
	HugoBuildDir  string `json:"hugoBuildDir"`
	GithubPageDir string `json:"githubPageDir"`
	PostsDir      string `json:"postsDir"`
}

type SourceConfig struct {
	BaseDir string   `json:"baseDir"`
	Exclude []string `json:"exclude"`
}

type Config struct {
	Blog BlogConfig   `json:"blog"`
	Src  SourceConfig `json:"src"`
}

//+------------------------ util functions -----------------------

func IsDir(path string) bool {
	s, err := os.Stat(path)
	if err != nil {
		return false
	}
	return s.IsDir()
}

func IsIncludeElement(element string, slice []string) bool {
	for _, elem := range slice {
		if element == elem {
			return true
		}
	}
	return false
}

func Command(command string, args ...string) func(func(error)) {
	return func(onError func(err error)) {
		out, err := exec.Command(command, args...).Output()
		com := make([]string, len(args)+2)
		com[0] = "👉🏼"
		com[1] = command
		copy(com[2:], args)
		log.Default().Println(strings.Join(com, " "))
		if err != nil {
			onError(err)
			log.Fatalf("❌ An error occured!!!\n%s", err.Error())
		}
		if len(out) > 0 {
			log.Default().Println(fmt.Sprintf("✅ %s", string(out)))
		}
	}

}

func CheckFileState(filename string) bool {
	_, err := os.Stat(filename)
	if err != nil {
		return false
	}
	return true
}

//+-------------------------- business functions --------------------------

// 加载配置文件
// 配置项存放在用户家目录下的.autosync.config.json
func LoadConfig(path string) Config {
	config, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatalf("❌ Can't load config file")
	}
	myConfig := &Config{}
	if ok := json.Unmarshal(config, myConfig); ok != nil {
		log.Fatalf("❌ Error parsing config file")
	}
	return *myConfig
}

func UpdateGithubPage(config *Config) {
	log.Default().Println("📝 Building github page")
	os.Chdir(config.Blog.HugoBuildDir)
	Command("hugo")(func(e error) {})

	log.Default().Println("📝 Pushing github page")
	os.Chdir(config.Blog.GithubPageDir)
	Command("git", "add", ".")(func(e error) {})
	Command("git", "commit", "-m", "\"✅Update by autosync\"")(func(e error) {})
	Command("git", "push", "-u", "origin", "main")(func(e error) {})

	log.Default().Println("📝 Pushing project")
	os.Chdir(config.Blog.ProjectDir)
	Command("git", "add", ".")(func(e error) {})
	Command("git", "commit", "-m", "\"✅Update by autosync\"")(func(e error) {})
	Command("git", "push", "-u", "origin", "main")(func(e error) {})
}

// 收集Obsidian下所有的markdown文件
// markdown文件必须以.md结尾才可以被扫描到
// 搜集之后的结果将会被放在files中
func CollectMarkdownFiles(config *Config, entry string, files *[]string) {
	if IsDir(entry) {
		fs, err := ioutil.ReadDir(entry)
		if err != nil {
			return
		}
		for _, file := range fs {
			// 过滤文件
			ok := IsIncludeElement(file.Name(), config.Src.Exclude)
			if ok {
				continue
			}
			CollectMarkdownFiles(config, fmt.Sprintf("%s%s%s", entry, string(os.PathSeparator), file.Name()), files)
		}
	} else {
		// 检查是否是markdown文件
		split := strings.Split(entry, ".")
		if IsIncludeElement("md", split) {
			*files = append(*files, entry)
		}
		return
	}
}

// 过滤出待上传的markdown的文件
// 待上传的文件需要在文件结尾添加#pub#
func FilterNotPublishedMarkdownFile(files []string) []string {
	res := make([]string, 0, len(files))
	for _, filename := range files {
		split := strings.Split(filename, "#")
		if IsIncludeElement("pub", split) {
			res = append(res, filename)
		}
	}
	return res
}

func PrepareBlogPost(config *Config, filenames []string) {
	for _, filename := range filenames {
		// 生成新的文件名，除去#pub#
		newPath := strings.Replace(filename, "#pub#", "", 1)
		base := path.Base(newPath)
		log.Default().Println(newPath)
		// 使用hugo创建
		os.Chdir(config.Blog.HugoBuildDir)
		dest := path.Join(config.Blog.PostsDir, base)
		Command("hugo", "new", strings.Join([]string{"posts", string(os.PathSeparator), base}, ""))(func(e error) {
			_ = os.Remove(dest)
		})

		// 写入使用hugo创建的文件中
		srcByte, err := ioutil.ReadFile(filename)
		if err != nil {
			_ = os.Remove(dest)
			log.Fatalf("❌ An error occured while reading source file\n%s", err.Error())
		}

		err = ioutil.WriteFile(dest, srcByte, 0330)
		if err != nil {
			_ = os.Remove(dest)
			log.Fatalf("❌ An error occured while writing source file\n%s", err.Error())
		}
		// 将原文件重命名
		_ = os.Rename(filename, newPath)
	}
}

func main() {
	userDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("❌ Can't read user home dir")
	}
	path := path.Join(userDir, ".autosync.config.json")
	stat, err := os.Stat(path)
	if err != nil || stat.IsDir() {
		log.Fatalf("❌ Config file not found")
	}

	config := LoadConfig(path)
	files := make([]string, 0, 10)
	CollectMarkdownFiles(&config, config.Src.BaseDir, &files)
	notPublished := FilterNotPublishedMarkdownFile(files)
	PrepareBlogPost(&config, notPublished)
	if len(notPublished) > 0 {
		UpdateGithubPage(&config)
	} else {
		log.Default().Println("📉 Noting to do")
	}
}
