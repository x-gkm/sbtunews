package sbtunews

import (
	"context"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/goodsign/monday"
)

type postItemDTO struct {
	Title       string   `selector:"a"`
	Link        string   `selector:"a" attr:"href"`
	Description string   `selector:".description"`
	Image       string   `selector:"img" attr:"src"`
	Stats       []string `selector:"li"`
}

type PostItem struct {
	Title       string    `json:"title"`
	Link        string    `json:"link"`
	Description string    `json:"description,omitempty"`
	Image       string    `json:"image,omitempty"`
	Views       int       `json:"views"`
	Date        time.Time `json:"date"`
}

func Recent(ctx context.Context) ([]*PostItem, error) {
	c := colly.NewCollector(
		colly.AllowedDomains(
			"sivas.edu.tr",
			"www.sivas.edu.tr",
			"muhendislik.sivas.edu.tr",
			"oidb.sivas.edu.tr",
		),
		colly.URLFilters(
			regexp.MustCompile(`/tum-haberler`),
		),
		colly.Async(),
		colly.StdlibContext(ctx),
	)

	locationTR, err := time.LoadLocation("Europe/Istanbul")
	if err != nil {
		return nil, err
	}

	ch := make(chan *PostItem)
	stopCh := make(chan struct{})
	errCh := make(chan error)

	c.OnHTML(`.post-item`, func(e *colly.HTMLElement) {
		var postDTO postItemDTO
		e.Unmarshal(&postDTO)

		views, err := strconv.Atoi(postDTO.Stats[1])
		if err != nil {
			errCh <- err
			return
		}

		date, err := monday.ParseInLocation("Monday 02 January 2006 15:04", postDTO.Stats[0], locationTR, monday.LocaleTrTR)
		if err != nil {
			errCh <- err
			return
		}

		ch <- &PostItem{
			Title:       postDTO.Title,
			Link:        postDTO.Link,
			Description: postDTO.Description,
			Image:       strings.ReplaceAll(postDTO.Image, "/thumbs/120/", "/"),
			Views:       views,
			Date:        date,
		}
	})

	c.Visit("https://www.sivas.edu.tr/tum-haberler")
	c.Visit("https://muhendislik.sivas.edu.tr/tum-haberler")
	c.Visit("https://oidb.sivas.edu.tr/tum-haberler")

	go func() {
		c.Wait()
		stopCh <- struct{}{}
	}()

	var result []*PostItem

Loop:
	for {
		select {
		case p := <-ch:
			result = append(result, p)
		case <-stopCh:
			break Loop
		case err := <-errCh:
			return nil, err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[j].Date.Before(result[i].Date)
	})

	return result, nil
}
