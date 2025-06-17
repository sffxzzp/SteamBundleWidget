package handler

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/anaskhan96/soup"
)

type (
	SteamBundle struct {
		Url string

		Name  string
		Price struct {
			Current  string
			Original string
		}
		Discount struct {
			Base    string
			Current string
		}
		Items []struct {
			ID   string
			Type string
			Name string
			Pic  string
		}
	}
)

func httpGet(url string) ([]byte, bool) {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, true
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; bingbot/2.0; +http://www.bing.com/bingbot.htm)")
	req.Header.Set("Cookie", "birthtime=-377705146002; lastagecheckage=1-1-1900; wants_mature_content=1;")
	resp, err := client.Do(req)
	if err != nil {
		return nil, true
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		data, _ := io.ReadAll(resp.Body)
		return data, false
	} else {
		return nil, true
	}
}

func getDOMText(doc *soup.Root, args ...string) string {
	elem := doc.Find(args...)
	if elem.Error != nil {
		return ""
	}
	text := elem.Text()
	return text
}

func newSteamBundle(bundleID string, region string, language string) *SteamBundle {
	return &SteamBundle{
		Url: fmt.Sprintf("https://store.steampowered.com/bundle/%s/?cc=%s&l=%s", bundleID, region, language),
	}
}

func (s *SteamBundle) getPageData(w *http.ResponseWriter) bool {
	resp, err := httpGet(s.Url)
	if err {
		(*w).WriteHeader(http.StatusInternalServerError)
		fmt.Fprint((*w), "Error: Network Error")
		return false
	}
	doc := soup.HTMLParse(string(resp))
	s.Name = getDOMText(&doc, "h2", "class", "pageheader")
	if s.Name == "" {
		(*w).WriteHeader(http.StatusNotFound)
		fmt.Fprint((*w), "Error: Bundle not found")
		return false
	}
	s.Price.Original = getDOMText(&doc, "div", "class", "discount_original_price")
	s.Price.Current = getDOMText(&doc, "div", "class", "discount_final_price")
	s.Discount.Base = getDOMText(&doc, "div", "class", "bundle_base_discount")
	s.Discount.Current = getDOMText(&doc, "div", "class", "discount_pct")
	Items := doc.Find("div", "class", "page_content").Find("div", "class", "game_description_column").FindAll("div", "class", "bundle_package_item")
	for _, it := range Items {
		item := it.Find("div", "class", "tab_item")
		itemKey := strings.Split(item.Attrs()["data-ds-itemkey"], "_")
		id := strings.ToLower(itemKey[1])
		itemType := strings.ToLower(itemKey[0])
		pic := fmt.Sprintf("https://media.steampowered.com/steam/%ss/%s/capsule_sm_120.jpg", itemType, id)
		capImg := item.Find("div", "class", "tab_item_cap").Find("img", "class", "tab_item_cap_img")
		if capImg.Error == nil {
			if src, ok := capImg.Attrs()["src"]; ok {
				pic = strings.Replace(strings.Split(src, "?")[0], "184x69", "sm_120", 1)
			}
		}
		s.Items = append(s.Items, struct {
			ID   string
			Type string
			Name string
			Pic  string
		}{
			ID:   id,
			Type: itemType,
			Name: item.Find("div", "class", "tab_item_name").Text(),
			Pic:  pic,
		})
	}
	return true
}

func (s *SteamBundle) render(w *http.ResponseWriter) {
	json, err := json.Marshal(s)
	if err != nil {
		(*w).WriteHeader(http.StatusInternalServerError)
		fmt.Fprint((*w), "Error: Json marshal error")
		return
	}
	(*w).Header().Set("Content-Type", "application/json")
	(*w).Header().Set("Cache-Control", "public, max-age=14400, s-maxage=14400, stale-while-revalidate=14400")
	(*w).Write(json)
}

func Handler(w http.ResponseWriter, r *http.Request) {
	BundleID := r.URL.Query().Get("BundleID")
	if BundleID == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "Error: BundleID is required")
		return
	}
	Region := r.URL.Query().Get("Region")
	if Region == "" {
		Region = "cn"
	}
	Language := r.URL.Query().Get("Language")
	if Language == "" {
		Language = "schinese"
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	bundle := newSteamBundle(BundleID, Region, Language)
	if bundle.getPageData(&w) {
		bundle.render(&w)
	}
}