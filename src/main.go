// DOC:
// https://joplinapp.org/api/references/rest_api/
// https://joplinapp.org/api/references/rest_api/#resources

package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type Item struct {
	ID string `json:"id"` // resource ID / note ID
}

// response need to be parsed
type joplinResponse struct {
	Error string `json:"error"`
	Items []Item `json:"items"`
	More  bool   `json:"has_more"`
}

type Req struct {
	port  int    // joplin Web Clipper service port
	token string // joplin token
}

func readRespBody(method, url string, v any) error {
	client := http.Client{
		Timeout: 3 * time.Second,
	}

	req, err := http.NewRequest(method, url, http.NoBody)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(v)
	// resp.Body 为空的时候, Unmarshal() 会报 EOF. Delete resources 成功之后 resp.Body 为空.
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}

	return nil
}

// DOC: Gets all resources.
// https://joplinapp.org/api/references/rest_api/#get-resources
// https://joplinapp.org/api/references/rest_api/#pagination
// returns attachments IDs
func getAllResources(req Req) (resourcesIDs map[string]struct{}, err error) {
	resourcesIDs = make(map[string]struct{})
	var mark = true
	for page := 1; mark; page++ {
		// GET request:
		// - limit: max restricted to 100.
		// - sort: by id.
		// - page: start from 1.
		// - fields: columns.
		url := fmt.Sprintf("http://localhost:%d/resources?token=%s&fields=id&order_by=id&limit=100&page=%d", req.port, req.token, page)
		var resp joplinResponse
		err := readRespBody("GET", url, &resp)
		if err != nil {
			log.Println(err)
			return nil, err
		}

		// joplin server return error.
		if resp.Error != "" {
			log.Println(resp.Error)
			return nil, errors.New(resp.Error)
		}

		for _, item := range resp.Items {
			resourcesIDs[item.ID] = struct{}{}
		}

		// 判断后续是否有更多的 resources.
		mark = resp.More
	}

	return resourcesIDs, nil
}

// DOC: Gets the notes (IDs) associated with a resource.
// https://joplinapp.org/api/references/rest_api/#get-resources-id-notes
func filterResources(req Req, resources map[string]struct{}) error {
	for id := range resources {
		url := fmt.Sprintf("http://localhost:%d/resources/%s/notes?token=%s&fields=id", req.port, id, req.token)

		var resp joplinResponse
		err := readRespBody("GET", url, &resp)
		if err != nil {
			log.Println(err)
			return err
		}

		// joplin server return error.
		if resp.Error != "" {
			log.Println(resp.Error)
			return errors.New(resp.Error)
		}

		// 如果 items 不存在, 说明引用该 resources 的 note 不存在.
		if len(resp.Items) > 0 {
			// 从 map 中删除
			delete(resources, id)
		}
	}

	return nil
}

// 根据 resources id 删除无用的 resources.
// Delete "http://localhost:port/resources/:id?token=Token"
func deleteResources(req Req, resources map[string]struct{}) error {
	for id := range resources {
		url := fmt.Sprintf("http://localhost:%d/resources/%s?token=%s", req.port, id, req.token)

		var resp joplinResponse
		err := readRespBody("DELETE", url, &resp)
		if err != nil {
			log.Println(err)
			return err
		}

		if resp.Error != "" {
			// if error add to "failToDelete" slice.
			log.Printf("delete %s error: %s\n", id, resp.Error)
			return errors.New(resp.Error)
		}
	}

	return nil
}

func main() {
	log.SetFlags(log.Llongfile)

	var port = flag.Int("p", 41184, "joplin Web Clipper service port")
	var token = flag.String("t", "", "joplin Web Clipper Authorization token")
	flag.Parse()

	if *token == "" {
		log.Println("token is empty")
		return
	}

	if *port > 65535 || *port < 0 {
		log.Println("port is invalid")
		return
	}

	req := Req{
		port:  *port,
		token: *token,
	}

	resources, err := getAllResources(req)
	if err != nil {
		return
	}

	err = filterResources(req, resources)
	if err != nil {
		return
	}

	if len(resources) < 1 {
		fmt.Println("no unused attachments")
		return
	}

	fmt.Println("unused attachments:")
	for id := range resources {
		fmt.Println("  - " + id)
	}
	fmt.Println("view these attachments in 'Tools > Note attachments'")

	// prompt delete resources
	fmt.Print("delete these resources? [Yes/no]: ")
	input, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		log.Println(err)
		return
	}
	input = strings.TrimSuffix(input, "\n")

	if input != "yes" && input != "Yes" {
		return
	}

	err = deleteResources(req, resources)
	if err != nil {
		return
	}
}
