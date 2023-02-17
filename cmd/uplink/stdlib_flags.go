// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"encoding/json"
	"flag"
	"regexp"
	"strconv"
	"time"

	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"
)

type stdlibFlags struct {
	fs *flag.FlagSet
}

func newStdlibFlags(fs *flag.FlagSet) *stdlibFlags {
	return &stdlibFlags{
		fs: fs,
	}
}

func (s *stdlibFlags) Setup(f clingy.Flags) {
	// we use the Transform function to store the value as a side
	// effect so that we can return an error if one occurs through
	// the expected clingy pipeline.
	s.fs.VisitAll(func(fl *flag.Flag) {
		name, _ := flag.UnquoteUsage(fl)
		f.Flag(fl.Name, fl.Usage, fl.DefValue,
			clingy.Advanced,
			clingy.Type(name),
			clingy.Transform(func(val string) (string, error) {
				return "", fl.Value.Set(val)
			}),
		)
	})
}

// parseHumanDate parses command-line flags which accept relative and absolute datetimes.
// It can be passed to clingy.Transform to create a clingy.Option.
func parseHumanDate(date string) (time.Time, error) {
	return parseHumanDateInLocation(date, time.Now().Location())
}

var durationWithDay = regexp.MustCompile(`(\+|-)(\d+)d`)

func parseHumanDateInLocation(date string, loc *time.Location) (time.Time, error) {
	switch {
	case date == "none":
		return time.Time{}, nil
	case date == "":
		return time.Time{}, nil
	case date == "now":
		return time.Now(), nil
	case date[0] == '+' || date[0] == '-':
		dayDuration := durationWithDay.FindStringSubmatch(date)
		if len(dayDuration) > 0 {
			days, _ := strconv.Atoi(dayDuration[2])
			if dayDuration[1] == "-" {
				days *= -1
			}
			return time.Now().Add(time.Hour * time.Duration(days*24)), nil
		}

		d, err := time.ParseDuration(date)
		return time.Now().Add(d), errs.Wrap(err)
	default:
		t, err := time.ParseInLocation(time.RFC3339, date, time.Now().Location())
		if err == nil {
			return t, nil
		}

		// shorter version of RFC3339
		for _, format := range []string{"2006-01-02T15:04:05", "2006-01-02T15:04", "2006-01-02"} {
			t, err := time.ParseInLocation(format, date, loc)
			if err == nil {
				return t, nil
			}
			time.Now().Location()
		}
		d, err := time.ParseDuration(date)
		if err == nil {
			return time.Now().Add(d), nil
		}
		return time.Time{}, err
	}
}

// parseJSON parses command-line flags which accept JSON string.
// It can be passed to clingy.Transform to create a clingy.Option.
func parseJSON(jsonString string) (map[string]string, error) {
	if len(jsonString) > 0 {
		var jsonValue map[string]string
		err := json.Unmarshal([]byte(jsonString), &jsonValue)
		if err != nil {
			return nil, err
		}
		return jsonValue, nil
	}
	return nil, nil
}
