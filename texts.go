package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"
	"unicode"
)

type PostsUnit struct {
	Board, Thread string
}

var PostsCache = map[PostsUnit][]string{}
var PostsMu sync.Mutex

type Chain struct {
	First []string
	Next  map[string][]string
}

func NewChain(texts []string) *Chain {
	firstWords := make([]string, 0)
	nextWords := make(map[string][]string)

	for _, text := range texts {
		f := strings.Split(text, " ")
		f = Filter(f, func(s string) bool {
			return Any([]rune(s), func(r rune) bool {
				return !unicode.IsSpace(r)
			})
		})
		if len(f) == 0 {
			continue
		}
		firstWords = append(firstWords, f[0])
		for i, word := range f {
			if i+1 != len(f) {
				nextWords[word] = append(nextWords[word], f[i+1])
			}
		}
	}
	return &Chain{
		First: firstWords,
		Next:  nextWords,
	}
}

func (chain *Chain) BuildText(maxlen int) string {
	if len(chain.First) == 0 {
		return ""
	}

	cur := chain.First[rand.Intn(len(chain.First))]
	result := []string{cur}

	for i := 0; i < maxlen && len(chain.Next[cur]) != 0; i++ {
		cur = chain.Next[cur][rand.Intn(len(chain.Next[cur]))]
		result = append(result, cur)
	}

	return strings.Join(result, " ")
}

func RemoveTags(text string) string {
	replacer := strings.NewReplacer(
		"&quot;", "\"",
		" (OP)", "",
		"<br>", "\n",
		"&gt;", ">",
		"&#47;", "/",
	)

	text = replacer.Replace(text)

	runes := []rune(text)
	tag := false

	result := make([]rune, 0)

	for _, r := range runes {
		if r == '>' && tag {
			tag = false
			continue
		}
		if r == '<' && !tag {
			tag = true
		}
		if tag {
			continue
		}
		result = append(result, r)
	}

	return string(result)
}

func GetPosts(board string, thread string) ([]string, error) {
	PostsMu.Lock()
	defer PostsMu.Unlock()

	unit := PostsUnit{board, thread}
	if PostsCache[unit] != nil {
		return PostsCache[unit], nil
	}

	url := fmt.Sprintf(
		"https://2ch.su/%s/res/%s.json",
		board,
		thread,
	)

	req := Request{
		Url:     url,
		Timeout: time.Second * 30,
	}

	resp, err := req.Perform()
	if err != nil {
		return nil, err
	}

	var posts struct {
		Threads []struct {
			Posts []struct {
				Comment string
			}
		}
	}

	json.Unmarshal(resp, &posts)

	if len(posts.Threads) == 0 {
		return nil, errors.New("указанный тред не найден!")
	}
	if len(posts.Threads[0].Posts) == 0 {
		return nil, errors.New("не найдено ни одного поста!")
	}
	for _, com := range posts.Threads[0].Posts {
		PostsCache[unit] = append(
			PostsCache[unit],
			RemoveTags(com.Comment),
		)
	}

	return PostsCache[unit], nil
}
