package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/gocolly/colly/v2"
	"google.golang.org/api/option"
	"google.golang.org/api/translate/v2"
)

var (
	apiKey      = "YOUR_API_KEY" // Google Cloud Translation APIのAPIキーを入力
	baseURL     string
	outputDir   string
	recursive   int
	targetLang  = "ja"
	visitedURLs = map[string]bool{}
)

// 翻訳関数
func trans(s *translate.Service, text string) (string, error) {
	translations, err := s.Translations.List([]string{text}, targetLang).Do()
	if err != nil {
		return "", err
	}

	if len(translations.Translations) > 0 {
		return translations.Translations[0].TranslatedText, nil
	}

	return "", fmt.Errorf("no translation found")
}

func main() {
	// 引数の定義
	flag.StringVar(&outputDir, "output-dir", "./output", "The output directory for translated HTML/CSS files")
	flag.StringVar(&baseURL, "url", "", "The URL of the website to translate")
	flag.IntVar(&recursive, "recursive", 1, "The number of levels to follow links from the base URL")
	flag.Parse()

	// 出力ディレクトリを作成
	err := os.MkdirAll(outputDir, 0755)
	if err != nil {
		fmt.Println("Error creating output directory:", err)
		return
	}

	// コレクターの作成
	c := colly.NewCollector(
		colly.MaxDepth(recursive),
		colly.Async(true),
	)

	// 翻訳サービスの作成
	translationClient, err := translate.NewService(context.Background(), option.WithAPIKey(apiKey))
	if err != nil {
		fmt.Println("Error creating translation service:", err)
		return
	}

	// スクレイピングの前処理
	c.OnHTML("*", func(e *colly.HTMLElement) {
		e.ForEach("*", func(_ int, e *colly.HTMLElement) {
			text := strings.TrimSpace(e.Text)
			if text != "" {
				translatedText, err := trans(translationClient, text)
				if err == nil {
					e.Text = translatedText
				} else {
					fmt.Printf("Failed to translate text: %s\n", err)
				}
			}
		})
	})

	// スクレイピング後の処理
	c.OnScraped(func(r *colly.Response) {
		outputPath := filepath.Join(outputDir, filepath.Base(r.Request.URL.Path))
		err := ioutil.WriteFile(outputPath, []byte(r.Body), 0644)
		if err != nil {
			fmt.Printf("Failed to write file: %s\n", err)
		} else {
			fmt.Printf("Translated page saved to: %s\n", outputPath)
		}
	})
	// リンクの処理
	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		link := e.Request.AbsoluteURL(e.Attr("href"))
		// 同じドメインのURLのみクロール
		if isSameDomain(baseURL, link) && !visitedURLs[link] {
			visitedURLs[link] = true
			err := e.Request.Visit(link)
			if err != nil {
				fmt.Printf("Error visiting link: %s\n", err)
			}
		}
	})

	// エラー処理
	c.OnError(func(r *colly.Response, err error) {
		fmt.Printf("Request URL: %s failed with response: %d\n", r.Request.URL, r.StatusCode)
	})

	// 開始URLの訪問
	err = c.Visit(baseURL)
	if err != nil {
		fmt.Printf("Error visiting base URL: %s\n", err)
	}

	// 実行
	c.Wait()
}

func isSameDomain(base, link string) bool {
	baseDomain, err := url.Parse(base)
	if err != nil {
		return false
	}
	linkDomain, err := url.Parse(link)
	if err != nil {
		return false
	}
	return baseDomain.Host == linkDomain.Host
}
