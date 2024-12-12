package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
	"github.com/thoas/go-funk"
)

// Helper functions

type JobArguments struct {
	Source   string
	Targets  []string
	Distance int
	Sort     string
	Limit    int
	Proofs   bool
}

const defaultDistance = 2
const defaultLimit = 5
const defaultSort = "global"

func getArguments(request *nostr.Event, requireTargets bool) (JobArguments, []error) {
	var args JobArguments
	var errs []error
	var pubkeyAuthorized bool

	err := Db.QueryRow("SELECT EXISTS(SELECT 1 FROM authorized_keys WHERE pubkey = ?)", request.PubKey).Scan(&pubkeyAuthorized)
	if err != nil {
		errs = append(errs, err)
		return args, errs
	}
	if !pubkeyAuthorized {
		errs = append(errs, errors.New("unauthorized"))
		return args, errs
	}

	source := findParamValues(request, "source")
	if len(source) > 0 {
		if strings.HasPrefix(source[0], "npub") {
			_, pubkey, err := nip19.Decode(source[0])
			if err != nil {
				errs = append(errs, errors.New("error decoding source key: "+source[0]))
			} else {
				args.Source = pubkey.(string)
			}
		} else {
			args.Source = source[0]
		}
	} else {
		// Default to signing pubkey
		args.Source = request.PubKey
	}

	if requireTargets {
		targets := findParamValues(request, "target")
		if len(targets) > 0 {
			for _, target := range targets {
				if strings.HasPrefix(target, "npub") {
					_, pubkey, err := nip19.Decode(target)
					if err != nil {
						errs = append(errs, errors.New("error decoding target key: "+target))
					} else {
						args.Targets = append(args.Targets, pubkey.(string))
					}
				} else {
					args.Targets = append(args.Targets, target)
				}
			}
		} else {
			errs = append(errs, errors.New("must supply targets"))
		}
	}

	distance := findParamValues(request, "distance")
	if len(distance) > 0 {
		d, err := strconv.Atoi(distance[0])
		if err != nil {
			args.Distance = defaultDistance
		} else {
			args.Distance = d
		}
	} else {
		args.Distance = defaultDistance
	}

	limit := findParamValues(request, "limit")
	if len(limit) > 0 {
		l, err := strconv.Atoi(limit[0])
		if err != nil {
			args.Limit = defaultLimit
		} else {
			args.Limit = l
		}
	} else {
		args.Limit = defaultLimit
	}

	sort := findParamValues(request, "sort")
	if len(sort) > 0 && sort[0] == "personalized" {
		args.Sort = "personalized"
	} else {
		args.Sort = defaultSort
	}

	proofs := findParamValues(request, "proofs")
	args.Proofs = len(proofs) > 0 && proofs[0] == "true"

	return args, errs
}

func findParamValues(request *nostr.Event, name string) []string {
	filtered := funk.Filter(request.Tags, func(tag nostr.Tag) bool {
		return tag.Key() == "param" && tag.Value() == name
	}).([]nostr.Tag)

	return funk.Map(filtered, func(tag nostr.Tag) string {
		return tag[2]
	}).([]string)
}

func createEvent(result any, request *nostr.Event) nostr.Event {
	jsonBytes, jsonErr := json.Marshal(result)
	if jsonErr != nil {
		return createErrorEvent([]error{jsonErr}, request)
	}

	content := string(jsonBytes)
	event := nostr.Event{
		Content:   content,
		CreatedAt: nostr.Now(),
		Kind:      request.Kind + 1000,
		Tags: nostr.Tags{
			nostr.Tag{"e", request.ID},
			nostr.Tag{"p", request.PubKey},
		},
	}

	BunkerClient.SignEvent(context.Background(), &event)
	return event
}

func createErrorEvent(errs []error, request *nostr.Event) nostr.Event {
	tags := nostr.Tags{
		nostr.Tag{"e", request.ID},
		nostr.Tag{"p", request.PubKey},
	}
	for _, err := range errs {
		tags = tags.AppendUnique(nostr.Tag{"status", "error", err.Error()})
	}

	event := nostr.Event{
		Content:   "",
		CreatedAt: nostr.Now(),
		Kind:      7000,
		Tags:      tags,
	}
	BunkerClient.SignEvent(context.Background(), &event)
	// event.Sign(RelayPrivateKey)
	return event
}

func getEnv() func(k string, fallback ...string) (v string) {
	var env = make(map[string]string)

	for _, item := range os.Environ() {
		parts := strings.SplitN(item, "=", 2)
		env[parts[0]] = parts[1]
	}

	return func(k string, fallback ...string) (v string) {
		v = env[k]

		if v == "" && len(fallback) > 0 {
			v = fallback[0]
		}

		return v
	}
}
