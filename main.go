package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/handlers"
	cache "github.com/patrickmn/go-cache"
	"github.com/pkg/errors"
)

const CacheTime = time.Minute * 5

type Token struct {
	Token       string    `json:"token"`
	AccessToken string    `json:"access_token"`
	ExpiresIn   int       `json:"expires_in"`
	IssuedAt    time.Time `json:"issued_at"`
}

type metricsHandler struct {
	cache Cache
}

type Cache interface {
	Set(string, interface{}, time.Duration)
	Get(string) (interface{}, bool)
}

func main() {
	c := cache.New(CacheTime, CacheTime)

	http.Handle("/metrics", metricsHandler{cache: c})

	if err := http.ListenAndServe(fmt.Sprintf(":%d", 8080), handlers.CombinedLoggingHandler(os.Stdout, http.DefaultServeMux)); err != nil {
		panic(err)
	}
}

func getDockerToken(ctx context.Context) (*Token, error) {
	var response Token

	err := jsonGet(ctx, "https://auth.docker.io/token?service=registry.docker.io&scope=repository:ratelimitpreview/test:pull", &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

func getDockerRateLimit(ctx context.Context, token *Token) ([]byte, error) {
	if token == nil {
		return nil, errors.New("broken token")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, "https://registry-1.docker.io/v2/ratelimitpreview/test/manifests/latest", nil)
	if err != nil {
		return nil, errors.Wrap(err, "error building request")
	}

	req.Header.Add("Authorization", "Bearer "+token.Token)

	r, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "http error")
	}

	defer r.Body.Close()

	limit := strings.Split(r.Header.Get("RateLimit-Limit"), ";")
	remaining := strings.Split(r.Header.Get("RateLimit-Remaining"), ";")

	remainingI, _ := strconv.Atoi(remaining[0])
	limitI, _ := strconv.Atoi(limit[0])

	var labels []string
	if node, foundEnv := os.LookupEnv("NODENAME"); foundEnv {
		labels = append(labels, fmt.Sprintf(`nodename="%s"`, node))
	}
	labelsS := strings.Join(labels,",")

	out := ""
	out += fmt.Sprintf("dockerhub_remaining{%s} %d\n", labelsS, remainingI)
	out += fmt.Sprintf("dockerhub_limit{%s} %d\n", labelsS, limitI)
	out += fmt.Sprintf("dockerhub_used{%s} %d\n", labelsS, (limitI - remainingI))

	return []byte(out), nil
}

func jsonGet(ctx context.Context, u string, target interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return errors.Wrap(err, "error building request")
	}

	r, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "http error")
	}
	defer r.Body.Close()

	return json.NewDecoder(r.Body).Decode(target)
}

func (h metricsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-type", "text/plain")
	w.WriteHeader(http.StatusOK)

	data, err := getDockerRateLimitCached(r.Context(), h.cache)
	if err != nil {
		log.Fatalf("Error collecting data: %s", err)
	}

	fmt.Fprintf(w, "%s", data)
}

func getDockerRateLimitCached(ctx context.Context, c Cache) ([]byte, error) {
	if cached, inCache := c.Get("dockerhub"); inCache {
		return cached.([]byte), nil
	}

	token, err := getDockerToken(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error building request")
	}

	data, err := getDockerRateLimit(ctx, token)
	if err != nil {
		return nil, err
	}

	if c != nil {
		c.Set("dockerhub", data, CacheTime)
	}

	return data, nil
}
