package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"text/template"
	"time"

	"github.com/BurntSushi/toml"
)

var (
	conf    Config
	at      *AtCoderClient
	clarMap map[string]Clar

	tplSlackQuestion *template.Template
	tplSlackAnswer   *template.Template
)

func init() {
	clarMap = map[string]Clar{}
	tplSlackQuestion = template.Must(template.New("slackquestion").Parse(`[Clar No.{{.ID}}]
問題名：{{if .ProblemTitle}} <{{.ProblemURL}}|{{.ProblemTitle}}> {{else}} -{{end}}
ユーザ名：<{{.UserURL}}|{{.UserID}}>
質問：{{.ClarText}}
<{{.ReplyURL}}|質問に回答する>`))

	tplSlackAnswer = template.Must(template.New("slackanswer").Parse(`[Clar No.{{.ID}} に回答しました]
問題名：{{if .ProblemTitle}} <{{.ProblemURL}}|{{.ProblemTitle}}> {{else}} -{{end}}
ユーザ名：<{{.UserURL}}|{{.UserID}}>
質問：{{.ClarText}}
回答：{{.ResponseText}}
全体公開：{{.IsPublic}}
<{{.ReplyURL}}|回答を修正する>`))
}

type duration struct {
	time.Duration
}

func (d *duration) UnmarshalText(text []byte) error {
	var err error
	d.Duration, err = time.ParseDuration(string(text))
	return err
}

type Config struct {
	AtCoderURL      string
	AtCoderUserID   string
	AtCoderPass     string
	SlackWebhookURL string
	CheckInterval   string
}

type Clar struct {
	ID           string
	ProblemTitle string
	ProblemURL   string
	UserID       string
	UserURL      string
	ClarText     string
	ResponseText string
	IsPublic     string
	ReplyURL     string
}

func usage() {
	log.Fatalf("Usage: %s path/to/config", os.Args[0])
}

func postToSlack(text string) error {
	params, err := json.Marshal(struct {
		Text string `json:"text"`
	}{Text: text})

	if err != nil {
		return err
	}

	resp, err := http.PostForm(conf.SlackWebhookURL,
		url.Values{"payload": {string(params)}})
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf(resp.Status)
	}
	return nil
}

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	if _, err := toml.DecodeFile(os.Args[1], &conf); err != nil {
		log.Fatalln("設定ファイルの読み込みに失敗しました.", err)
	}
	interval, err := time.ParseDuration(conf.CheckInterval)
	if interval <= time.Second {
		interval = time.Second
	}
	log.Println(conf.AtCoderURL, "の監視を開始します.")
	at = newAtCoderClient(conf.AtCoderURL, conf.AtCoderUserID, conf.AtCoderPass)
	if err := at.Login(); err != nil {
		log.Fatalln("ログインに失敗しました.", err)
	}
	clars, err := at.GetClars()
	if err != nil {
		log.Fatalln("初回クラー取得に失敗しました.", err)
	} else {
		log.Println("初回クラー取得に成功しました.")
		log.Println(clars)
	}
	for _, clar := range clars {
		clarMap[clar.ID] = clar
	}
	for {
		time.Sleep(interval)
		clars, err = at.GetClars()
		if err != nil {
			log.Print("クラー取得に失敗しました.", err)
			log.Print("1分後にリトライします.")
			time.Sleep(1 * time.Minute)
			continue
		}
		for _, clar := range clars {
			c, ok := clarMap[clar.ID]
			if !ok || (clar.ResponseText != c.ResponseText) {
				var buf bytes.Buffer
				if ok {
					tplSlackAnswer.Execute(&buf, clar)
					log.Println("クラーに回答しました")
					log.Println(clar)
				} else {
					tplSlackQuestion.Execute(&buf, clar)
					log.Println("新しいクラーがきました", clar)
					log.Println(clar)
				}
				log.Println(buf.Len())
				if buf.Len() != 0 {
					err = postToSlack(buf.String())
					if err != nil {
						log.Println("Slackへの通知に失敗しました.", err)
					} else {
						clarMap[clar.ID] = clar
					}
				}
			}
		}
	}
}
