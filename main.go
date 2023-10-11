package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"

	goteamsnotify "github.com/atc0005/go-teams-notify/v2"
	"github.com/atc0005/go-teams-notify/v2/messagecard"
)

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

var (
	host             = getEnv("ELASTIC_HOST", "https://localhost:9200")
	username         = getEnv("ELASTIC_USERNAME", "elastic")
	password         = getEnv("ELASTIC_PASSWORD", "s3cr3t")
	index_name       = getEnv("ELASTIC_INDEX", "alerts")
	timestamp_field  = getEnv("ELASTIC_TIMESTAMP_FIELD", "@timestamp")
	tags_field       = getEnv("ELASTIC_TAGS_FIELD", "tags")
	event_type_field = getEnv("ELASTIC_EVENT_TYPE_FIELD", "event")
	channel          = getEnv("NOTIFY_CHANNEL", "msteams")
	webhook          = getEnv("NOTIFY_MSTEAMS_WEBHOOK", "http://unusable")

	freq_var            = getEnv("ALERT_INTERVAL", "300") // in seconds
	gte                 = fmt.Sprintf("now-%ss", freq_var)
	frequency, err_freq = strconv.Atoi(freq_var)

	dryrun_var         = getEnv("DRYRUN", "false")
	dryrun, err_dryrun = strconv.ParseBool(dryrun_var)
)

func queryElastic(mstClient *goteamsnotify.TeamsClient, client *elasticsearch.Client, elasticQuery string) {

	log.Printf("Query ElasticSearch on %s index each %s seconds", index_name, freq_var)

	res, err := client.Search(
		client.Search.WithIndex(index_name),
		client.Search.WithBody(strings.NewReader(elasticQuery)),
	)

	if err != nil {
		log.Fatalf("Error getting response: %s", err)
	}
	defer res.Body.Close()
	var r map[string]interface{}

	if res.IsError() {
		var e map[string]interface{}
		if err := json.NewDecoder(res.Body).Decode(&e); err != nil {
			log.Fatalf("Error parsing the response body: %s", err)
		} else {
			// Print the response status and error information.
			log.Fatalf("[%s] %s: %s",
				res.Status(),
				e["error"].(map[string]interface{})["type"],
				e["error"].(map[string]interface{})["reason"],
			)
		}
	}

	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		log.Fatalf("Error parsing the response body: %s", err)
	}
	// Print the response status, number of results, and request duration.
	log.Printf(
		"[%s] %d hits; took: %dms",
		res.Status(),
		int(r["hits"].(map[string]interface{})["total"].(map[string]interface{})["value"].(float64)),
		int(r["took"].(float64)),
	)

	// Trigger notifications foreach entry

	for _, hit := range r["hits"].(map[string]interface{})["hits"].([]interface{}) {
		doc := hit.(map[string]interface{})["_source"]
		doc_map := doc.(map[string]interface{})
		log.Printf(" → ID=%s, %s", hit.(map[string]interface{})["_id"], doc)

		// TODO: configurable mapping of values
		event := doc.(map[string]interface{})[event_type_field]
		timestamp := doc.(map[string]interface{})[timestamp_field]
		rule_name := doc.(map[string]interface{})["ruleName"]
		doc_count := doc.(map[string]interface{})["contextMatchingDocuments"]
		tags_value := doc.(map[string]interface{})[tags_field]
		tags := strings.Split(fmt.Sprintf("%v", tags_value), ",")

		event_icon := ""
		event_color := ""

		if event == "fired" {
			event_icon = "☢️"
			event_color = "#f54242"
		} else if event == "recovered" {
			event_icon = "✅"
			event_color = "#42f59b"
		} else {
			event_icon = "ℹ️"
			event_color = "#FFF"
		}

		msgCard := messagecard.NewMessageCard()
		msgCard.Title = fmt.Sprintf("%s Kibana Alert %s: %s ", event_icon, event, rule_name)
		msgCard.Text = fmt.Sprintf("On date **%s** %s events trigger rule **%s**",
			timestamp, doc_count, rule_name)
		msgCard.ThemeColor = event_color

		section := messagecard.NewSection()
		section.ActivityTitle = "Details"

		for k := range doc_map {
			value := fmt.Sprintf("%s", doc_map[k])
			if len(value) > 0 {
				fact := messagecard.NewSectionFact()
				fact.Name = k
				fact.Value = value
				section.Facts = append(section.Facts, *fact)
			}
		}
		msgCard.AddSection(section)

		// if tags create a section with them
		if len(tags) > 0 {
			section_tags := messagecard.NewSection()
			tags_string := strings.Join(tags, "`,`")
			section_tags.ActivityTitle = "Tags"
			section_tags.Text = fmt.Sprintf("`%s`", tags_string)
			msgCard.AddSection(section_tags)
		}

		if dryrun {
			log.Printf("%+v", msgCard)
		} else {
			if err := mstClient.Send(webhook, msgCard); err != nil {
				log.Printf("failed to send message: %v", err)
				os.Exit(1)
			}
		}

	}

}

func main() {

	// Initialize a new Microsoft Teams client.
	mstClient := goteamsnotify.NewTeamsClient()

	transport := http.DefaultTransport
	tlsClientConfig := &tls.Config{InsecureSkipVerify: true}
	transport.(*http.Transport).TLSClientConfig = tlsClientConfig

	cfg := elasticsearch.Config{
		Addresses: []string{
			host,
		},
		Username:  username,
		Password:  password,
		Transport: transport,
	}

	elasticQuery := fmt.Sprintf(`
	{
		"query": {
			"range": {
				"%s": {
					"gte": "%s"
				}
			}
		}
	}`, timestamp_field, gte)

	client, err := elasticsearch.NewClient(cfg)
	if err != nil {
		log.Panic(err)
	}

	for true {
		queryElastic(mstClient, client, elasticQuery)
		time.Sleep(time.Duration(frequency) * time.Second)
	}

}
