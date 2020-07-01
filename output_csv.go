package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"sort"
	"sync"

	"github.com/ikawalec/golicense/config"
	"github.com/ikawalec/golicense/license"
	"github.com/ikawalec/golicense/module"
	"github.com/pkg/errors"
)

type CSVOutput struct {
	// Path is the path to the file to write. This will be overwritten if
	// it exists.
	Path string

	// Config is the configuration (if any). This will be used to check
	// if a license is allowed or not.
	Config *config.Config

	modules map[*module.Module]interface{}
	lock    sync.Mutex
}

// Start implements Output
func (o *CSVOutput) Start(m *module.Module) {}

// Update implements Output
func (o *CSVOutput) Update(m *module.Module, t license.StatusType, msg string) {}

// Finish implements Output
func (o *CSVOutput) Finish(m *module.Module, l *license.License, err error) {
	o.lock.Lock()
	defer o.lock.Unlock()

	if o.modules == nil {
		o.modules = make(map[*module.Module]interface{})
	}

	o.modules[m] = l
	if err != nil {
		o.modules[m] = err
	}
}

type Record struct {
	Dependency string
	Version    string
	SPDX       string
	License    string
}

func (r *Record) ToRow() []string {
	return []string{
		r.Dependency,
		r.Version,
		r.SPDX,
		r.License,
	}
}

// Close implements Output
func (o *CSVOutput) Close() error {
	o.lock.Lock()
	defer o.lock.Unlock()

	f, err := os.Create(o.Path)
	if err != nil {
		return errors.Wrapf(err, "failed to create file: %s", o.Path)
	}

	w := csv.NewWriter(f)
	defer w.Flush()

	keys := make([]string, 0, len(o.modules))
	index := map[string]*module.Module{}
	licenses := map[string]interface{}{}

	for m, l := range o.modules {
		keys = append(keys, m.Path)
		index[m.Path] = m

		licenses[m.Path] = l
	}
	sort.Strings(keys)

	final := make([]Record, len(keys))

	for i, k := range keys {
		m := index[k]
		l := licenses[k]
		switch t := l.(type) {
		case error:
			final[i] = Record{
				Dependency: m.Path,
				Version:    m.Version,
				License:    "not found",
				SPDX:       "NOT-FOUND",
			}
		case *license.License:
			final[i] = Record{
				Dependency: m.Path,
				Version:    m.Version,
			}
			if t != nil {
				final[i].SPDX = t.SPDX
				final[i].License = t.Name
			} else {
				final[i].SPDX = "NOT-FOUND"
				final[i].License = "not-found"
			}
		default:
			return fmt.Errorf("unexpected license type: %T", t)
		}
	}

	headers := []string{"Dependency", "Version", "SPDX", "License"}
	err = w.Write(headers)
	if err != nil {
		return errors.Wrapf(err, "faild to write headers to csv")
	}

	for _, r := range final {
		err = w.Write(r.ToRow())
		if err != nil {
			return errors.Wrapf(err, "failed to write to csv")
		}
	}

	return nil
}
