package main

import (
	"fmt"
	"testing"
)

func TestGetAllRes(t *testing.T) {
	req := Req{
		port:  41184,
		token: "2288804904e251f046bb730df0fe60a8cf5ed0f30e0260f00da3feb032aa4fbbe7bc2a57261af926d0ef959b2a2a7b9fe4f2972f95ae4b7320ba7f0d7ca93aec",
	}
	resources, err := getAllResources(req)
	if err != nil {
		t.Error(err)
		return
	}

	err = filterResources(req, resources)
	if err != nil {
		t.Error(err)
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
	fmt.Println("delete these attachments in 'Tools > Note attachments'")
}
