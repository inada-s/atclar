package main

import (
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type AtCoderClient struct {
	URL    string
	UserID string
	Pass   string
	client *http.Client
}

func newAtCoderClient(url, user, password string) *AtCoderClient {
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	client.Timeout = 5 * time.Second
	return &AtCoderClient{
		URL:    url,
		UserID: user,
		Pass:   password,
		client: client,
	}
}

func (at *AtCoderClient) GetClars() ([]Clar, error) {
	resp, err := at.client.Get(at.URL + "/clarifications")
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(resp.Status)
	}
	defer resp.Body.Close()
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	var clars []Clar
	doc.Find("tbody").Find("tr").Each(func(_ int, tr *goquery.Selection) {
		var clar Clar
		tr.Find("td").Each(func(i int, td *goquery.Selection) {
			switch i {
			case 0:
				clar.ProblemTitle = td.Text()
				if path, ok := td.Find("a").Attr("href"); ok {
					clar.ProblemURL = at.URL + path
				}
			case 1:
				clar.UserID = td.Text()
				if path, ok := td.Find("a").Attr("href"); ok {
					clar.UserURL = at.URL + path
				}
			case 2:
				clar.ClarText = td.Text()
			case 3:
				clar.ResponseText = td.Text()
			case 4:
				clar.IsPublic = td.Text()
			case 7:
				if path, ok := td.Find("a").Attr("href"); ok {
					clar.ReplyURL = at.URL + path
					slash := strings.LastIndex(path, "/")
					if slash != -1 && slash+1 != len(path) {
						clar.ID = path[slash+1:]
					}
				}
			}
		})
		clars = append(clars, clar)
	})

	return clars, nil
}

func (at *AtCoderClient) Login() error {
	u, err := url.Parse(at.URL)
	if err != nil {
		return err
	}

	data := url.Values{"name": {at.UserID}, "password": {at.Pass}}
	resp, err := at.client.PostForm(at.URL+"/login", data)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf(resp.Status)
	}

	isOwner := false
	for _, cookie := range at.client.Jar.Cookies(u) {
		if cookie.Name == "__privilege" && cookie.Value == "owner" {
			isOwner = true
		}
	}

	if !isOwner {
		return fmt.Errorf("オーナー権限がありません.")
	}

	return nil
}
