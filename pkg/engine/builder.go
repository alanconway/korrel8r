// Copyright: This file is part of korrel8r, released under https://github.com/korrel8r/korrel8r/blob/main/LICENSE

// # Template Functions
//
//	query
//	    Executes its argument as a korrel8r query, returns []any.
//	    May return an error.
package engine

import (
	"errors"
	"fmt"
	"text/template"

	"maps"

	"github.com/Masterminds/sprig/v3"
	"github.com/korrel8r/korrel8r/pkg/config"
	"github.com/korrel8r/korrel8r/pkg/korrel8r"
	"github.com/korrel8r/korrel8r/pkg/rules"
	"github.com/korrel8r/korrel8r/pkg/unique"
)

// Builder initializes the state of an engine.
// Engine() returns the immutable engine instance.
type Builder struct {
	e   *Engine
	err error
}

func Build() *Builder {
	e := &Engine{
		domains:     map[string]korrel8r.Domain{},
		stores:      map[string]*stores{},
		rulesByName: map[string]korrel8r.Rule{},
	}
	e.templateFuncs = template.FuncMap{"query": e.query}
	maps.Copy(e.templateFuncs, sprig.TxtFuncMap())
	return &Builder{e: e}
}

func (b *Builder) Domains(domains ...korrel8r.Domain) *Builder {
	for _, d := range domains {
		switch b.e.domains[d.Name()] {
		case d: // Already present
		case nil:
			b.e.domains[d.Name()] = d
			b.e.stores[d.Name()] = newStores(b.e, d)
			if tf, ok := d.(interface{ TemplateFuncs() map[string]any }); ok {
				maps.Copy(b.e.templateFuncs, tf.TemplateFuncs())
			}
		default:
			b.err = fmt.Errorf("Duplicate domain name: %v", d.Name())
			return b
		}
	}
	return b
}

func (b *Builder) Stores(stores ...korrel8r.Store) *Builder {
	for _, s := range stores {
		if b.err != nil {
			return b
		}
		d := s.Domain()
		b.Domains(d)
		if b.err != nil {
			return b
		}
		b.err = b.e.stores[d.Name()].Add(&store{domain: d, Store: s})
	}

	return b
}

func (b *Builder) StoreConfigs(storeConfigs ...config.Store) *Builder {
	for _, sc := range storeConfigs {
		if b.err != nil {
			return b
		}
		d := b.getDomain(sc[config.StoreKeyDomain])
		if b.err != nil {
			return b
		}
		b.err = b.e.stores[d.Name()].Add(&store{domain: d, Original: maps.Clone(sc)})
	}
	return b
}

func (b *Builder) Rules(rules ...korrel8r.Rule) *Builder {
	for _, r := range rules {
		if b.err != nil {
			return b
		}
		r2 := b.e.rulesByName[r.Name()]
		if r2 != nil {
			b.err = fmt.Errorf("Duplicate rule name: %v", r.Name())
			return b
		}
		b.Domains(r.Start()[0].Domain(), r.Goal()[0].Domain())
		b.e.rulesByName[r.Name()] = r
		b.e.rules = append(b.e.rules, r)
	}
	return b
}

// Config an engine.Builder.
func (b *Builder) Config(configs config.Configs) *Builder {
	if b.err != nil {
		return b
	}
	for source, c := range configs {
		b.config(c.Source, &c)
		if b.err != nil {
			b.err = fmt.Errorf("%v: %w", source, b.err)
			return b
		}
	}
	return b
}

func (b *Builder) ConfigFile(file string) *Builder {
	cfg, err := config.Load(file)
	if err != nil {
		b.err = err
		return b
	}
	return b.Config(cfg)
}

// Engine returns the final engine, which can no longer be modified.
// The Builder must not be used after calling Engine()
func (b *Builder) Engine() (*Engine, error) {
	e := b.e
	b.e = nil
	// Create all stores to report problems early.
	for _, ss := range e.stores {
		ss.Ensure()
	}
	return e, b.err
}

func (b *Builder) config(source string, c *config.Config) {
	if b.err != nil {
		return
	}
	b.StoreConfigs(c.Stores...)
	for _, r := range c.Rules {
		b.rule(r)
		if b.err != nil {
			return
		}
	}
}

func (b *Builder) rule(r config.Rule) {
	if b.err != nil {
		return
	}
	defer func() {
		var e korrel8r.ClassNotFoundError
		if errors.As(b.err, &e) {
			// Skip rules with invalid classes but don't fail the configuration.
			// Different clusters may have different resource sets, use the rules that are valid.
			// TODO: Need to distinguish class-not-found from cluster-connection-broken.
			log.Error(b.err, "Skipping rule", "rule", r.Name)
			b.err = nil
		}
	}()
	start := b.classes(&r.Start)
	if b.err != nil {
		return
	}
	goal := b.classes(&r.Goal)
	if b.err != nil {
		return
	}
	var tmpl *template.Template
	tmpl, b.err = b.e.NewTemplate(r.Name).Parse(r.Result.Query)
	if b.err != nil {
		return
	}
	b.Rules(rules.NewTemplateRule(start, goal, tmpl))
}

func (b *Builder) classes(spec *config.ClassSpec) []korrel8r.Class {
	d := b.getDomain(spec.Domain)
	if b.err != nil {
		return nil
	}
	list := unique.NewList[korrel8r.Class]()
	if len(spec.Classes) == 0 {
		all := d.Classes()
		if len(all) == 0 {
			b.err = fmt.Errorf("No classes found: %w", korrel8r.ClassNotFoundError{Domain: d})
			return nil
		}
		list.Append(d.Classes()...) // Missing class list means all classes in domain.
	} else {
		for _, class := range spec.Classes {
			c := d.Class(class)
			if c == nil {
				b.err = korrel8r.ClassNotFoundError{Class: class, Domain: d}
				return nil
			}
			list.Append(c)
		}
	}
	return list.List
}

func (b *Builder) getDomain(name string) (d korrel8r.Domain) {
	d, b.err = b.e.DomainErr(name)
	return
}
