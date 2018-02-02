package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"

	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	matcher = kingpin.Flag("matcher", "AWS format matcher for response status codes. E.g. 200-399,429").Default("200-399").String()
	ca      = kingpin.Flag("ca", "CA bundle").String()
	url     = kingpin.Arg("url", "URL to query as part of healthcheck").Required().String()
)

type matcherFunc func(int) bool

func parseMatchers(matcher string) ([]matcherFunc, error) {
	matchers := []matcherFunc{}

	for _, m := range strings.Split(matcher, ",") {
		bounds := strings.Split(m, "-")
		vals := make([]int, len(bounds))

		for i, bound := range bounds {
			if val, err := strconv.Atoi(bound); err != nil {
				return matchers, err
			} else {
				vals[i] = val
			}
		}

		if len(vals) == 1 {
			matchers = append(matchers, func(i int) bool { return i == vals[0] })
		} else if len(vals) == 2 {
			matchers = append(matchers, func(i int) bool {
				return i >= vals[0] && i <= vals[1]
			})
		} else {
			return matchers, fmt.Errorf("Could not parse bound '%s', either needs to be single value '200' or range '200-399'", m)
		}
	}

	return matchers, nil
}

func execMatchers(matchers []matcherFunc, val int) bool {
	matches := false
	for _, matcher := range matchers {
		if matcher(val) {
			matches = true
			break
		}
	}
	return matches
}

func loadCertPool(path string) (*x509.CertPool, error) {
	bundle, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	pool := x509.NewCertPool()

	if !pool.AppendCertsFromPEM(bundle) {
		return nil, fmt.Errorf("Could not add certs from bundle")
	}

	return pool, nil
}

func main() {
	kingpin.Parse()

	matchers, err := parseMatchers(*matcher)
	if err != nil {
		panic(err)
	}

	var client *http.Client
	if ca != nil && *ca != "" {
		certPool, err := loadCertPool(*ca)
		if err != nil {
			panic(err)
		}

		tlsConfig := &tls.Config{
			RootCAs: certPool,
		}

		client = &http.Client{
			Transport: &http.Transport{TLSClientConfig: tlsConfig},
		}
	} else {
		client = &http.Client{}
	}

	exitCode := 0
	if response, err := client.Get(*url); err != nil || !execMatchers(matchers, response.StatusCode) {
		exitCode = 1
	}

	os.Exit(exitCode)
}
