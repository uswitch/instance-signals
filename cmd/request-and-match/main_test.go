package main

import (
	"testing"
)

func TestParseMatchers(t *testing.T) {

	_, err := parseMatchers("200-399,429")

	if err != nil {
		t.Fatal(err)
	}

}

func TestExecMatchers(t *testing.T) {

	matchers, _ := parseMatchers("200-399,429")

	if !execMatchers(matchers, 200) {
		t.Fatal("200 should be matched")
	}

	if !execMatchers(matchers, 204) {
		t.Fatal("200 should be matched")
	}

	if !execMatchers(matchers, 399) {
		t.Fatal("200 should be matched")
	}

	if !execMatchers(matchers, 429) {
		t.Fatal("429 should be fine")
	}

	if execMatchers(matchers, 404) {
		t.Fatal("404 should fail")
	}
}
