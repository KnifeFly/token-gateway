package contract_test

import (
	"encoding/json"
	"testing"
	"time"

	tasksvc "github.com/KnifeFly/token-gateway/internal/task"
	"github.com/KnifeFly/token-gateway/pkg/tokenusage"
)

func TestMediaFileObjectIsTransientInputAsset(t *testing.T) {
	file := &tasksvc.FileAsset{
		ID:           "file_1",
		FileName:     "input.png",
		OriginalName: "input.png",
		SizeBytes:    2,
		MIMEType:     "image/png",
		Source:       "upload_base64",
		ContentHash:  "sha256:abc",
		Transient:    true,
		CreatedAt:    time.Unix(100, 0).UTC(),
	}

	object := tasksvc.FileObject(file)
	if object["transient"] != true || object["content_hash"] != "sha256:abc" || object["source"] != "upload_base64" {
		t.Fatalf("file object = %#v", object)
	}
	if _, ok := object["file_url"]; ok {
		t.Fatalf("file_url should be omitted for non-storage assets: %#v", object)
	}
	if _, ok := object["download_url"]; ok {
		t.Fatalf("download_url should be omitted for non-storage assets: %#v", object)
	}
}

func TestMediaTaskObjectExposesProviderResultContract(t *testing.T) {
	result, _ := json.Marshal(map[string]any{
		"results": []string{"https://cdn.example/result.png"},
		"assets": []tasksvc.ResultAsset{{
			URL:      "https://cdn.example/result.png",
			Type:     "image",
			Provider: "replicate",
		}},
		"provider_metadata": map[string]string{"prediction_id": "pred_1"},
	})
	task := &tasksvc.Task{
		ID:             "task_1",
		Kind:           tasksvc.KindImageGeneration,
		MediaType:      "image",
		Model:          "image-public",
		Status:         tasksvc.StatusSucceeded,
		Progress:       100,
		ProviderType:   "replicate",
		ChannelID:      "channel_replicate",
		ProviderTaskID: "pred_1",
		Result:         result,
		Usage:          tokenusage.Actual{InputTokens: 1, OutputTokens: 2, TotalTokens: 3},
		CreatedAt:      time.Unix(100, 0).UTC(),
		Metadata:       map[string]string{"customer": "team-a"},
	}

	object := tasksvc.TaskObject(task)
	if object["provider_task_id"] != "pred_1" || object["provider_type"] != "replicate" || object["channel_id"] != "channel_replicate" {
		t.Fatalf("task object = %#v", object)
	}
	results, ok := object["results"].([]string)
	if !ok || len(results) != 1 || results[0] != "https://cdn.example/result.png" {
		t.Fatalf("results = %#v", object["results"])
	}
	assets, ok := object["assets"].([]tasksvc.ResultAsset)
	if !ok || len(assets) != 1 || assets[0].Provider != "replicate" {
		t.Fatalf("assets = %#v", object["assets"])
	}
	metadata, ok := object["provider_metadata"].(map[string]string)
	if !ok || metadata["prediction_id"] != "pred_1" {
		t.Fatalf("provider metadata = %#v", object["provider_metadata"])
	}
}
