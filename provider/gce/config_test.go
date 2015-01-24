// Copyright 2014 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package gce_test

import (
	jc "github.com/juju/testing/checkers"
	gc "gopkg.in/check.v1"

	"github.com/juju/juju/environs"
	"github.com/juju/juju/environs/config"
	"github.com/juju/juju/provider/gce"
	"github.com/juju/juju/testing"
)

type ConfigSuite struct {
	gce.BaseSuite

	config *config.Config
}

var _ = gc.Suite(&ConfigSuite{})

func (s *ConfigSuite) SetUpTest(c *gc.C) {
	s.BaseSuite.SetUpTest(c)

	cfg, err := testing.EnvironConfig(c).Apply(gce.ConfigAttrs)
	c.Assert(err, jc.ErrorIsNil)
	s.config = cfg
}

// configTestSpec defines a subtest to run in a table driven test.
type configTestSpec struct {
	// info describes the subtest.
	info string
	// insert holds attrs that should be merged into the config.
	insert testing.Attrs
	// remove has the names of attrs that should be removed.
	remove []string
	// expect defines the expected attributes in a success case.
	expect testing.Attrs
	// err is the error message to expect in a failure case.
	err string
}

func (ts configTestSpec) checkSuccess(c *gc.C, cfg *config.Config, err error) {
	if !c.Check(err, jc.ErrorIsNil) {
		return
	}

	attrs := cfg.AllAttrs()
	for field, value := range ts.expect {
		c.Check(attrs[field], gc.Equals, value)
	}
}

func (ts configTestSpec) checkFailure(c *gc.C, err error, msg string) {
	c.Check(err, gc.ErrorMatches, msg+": "+ts.err)
}

func (ts configTestSpec) checkAttrs(c *gc.C, attrs map[string]interface{}, cfg *config.Config) {
	for field, value := range cfg.UnknownAttrs() {
		c.Check(attrs[field], gc.Equals, value)
	}
}

func (ts configTestSpec) attrs() testing.Attrs {
	return gce.ConfigAttrs.Merge(ts.insert).Delete(ts.remove...)
}

func (ts configTestSpec) config(c *gc.C) *config.Config {
	attrs := ts.attrs()
	cfg, err := testing.EnvironConfig(c).Apply(attrs)
	c.Assert(err, jc.ErrorIsNil)
	return cfg
}

func (ts configTestSpec) attrFixes() testing.Attrs {
	if ts.err != "" {
		return nil
	}

	attrs := make(testing.Attrs)
	for field, value := range ts.insert {
		for _, immutable := range gce.ConfigImmutable {
			if field == immutable {
				attrs[field] = value
				break
			}
		}
	}
	return attrs
}

var newConfigTests = []configTestSpec{{
	info:   "client-id is required",
	remove: []string{"client-id"},
	err:    "client-id: expected string, got nothing",
}, {
	info:   "client-id cannot be empty",
	insert: testing.Attrs{"client-id": ""},
	err:    "client-id: must not be empty",
}, {
	info:   "private-key is required",
	remove: []string{"private-key"},
	err:    "private-key: expected string, got nothing",
}, {
	info:   "private-key cannot be empty",
	insert: testing.Attrs{"private-key": ""},
	err:    "private-key: must not be empty",
}, {
	info:   "client-email is required",
	remove: []string{"client-email"},
	err:    "client-email: expected string, got nothing",
}, {
	info:   "client-email cannot be empty",
	insert: testing.Attrs{"client-email": ""},
	err:    "client-email: must not be empty",
}, {
	info:   "region is required",
	remove: []string{"region"},
	err:    "region: expected string, got nothing",
}, {
	info:   "region cannot be empty",
	insert: testing.Attrs{"region": ""},
	err:    "region: must not be empty",
}, {
	info:   "project-id is required",
	remove: []string{"project-id"},
	err:    "project-id: expected string, got nothing",
}, {
	info:   "project-id cannot be empty",
	insert: testing.Attrs{"project-id": ""},
	err:    "project-id: must not be empty",
}, {
	info:   "image-endpoint is inserted if missing",
	remove: []string{"image-endpoint"},
	expect: testing.Attrs{"image-endpoint": "https://www.googleapis.com"},
}, {
	info:   "image-endpoint can be empty",
	insert: testing.Attrs{"image-endpoint": ""},
	// We do not expect the default value because
	// an explict call to config.Apply never happens.
	expect: testing.Attrs{"image-endpoint": ""},
}, {
	info:   "unknown field is not touched",
	insert: testing.Attrs{"unknown-field": 12345},
	expect: testing.Attrs{"unknown-field": 12345},
}}

func (*ConfigSuite) TestNewEnvironConfig(c *gc.C) {
	for i, test := range newConfigTests {
		c.Logf("test %d: %s", i, test.info)

		testConfig := test.config(c)
		environ, err := environs.New(testConfig)

		// Check the result
		if test.err != "" {
			test.checkFailure(c, err, "invalid config")
		} else {
			test.checkSuccess(c, environ.Config(), err)
		}
	}
}

// TODO(wwitzel3) refactor to provider_test file
func (*ConfigSuite) TestValidateNewConfig(c *gc.C) {
	for i, test := range newConfigTests {
		c.Logf("test %d: %s", i, test.info)

		testConfig := test.config(c)
		validatedConfig, err := gce.Provider.Validate(testConfig, nil)

		// Check the result
		if test.err != "" {
			test.checkFailure(c, err, "invalid config")
		} else {
			test.checkSuccess(c, validatedConfig, err)
		}
	}
}

// TODO(wwitzel3) refactor to the provider_test file
func (s *ConfigSuite) TestValidateOldConfig(c *gc.C) {
	for i, test := range newConfigTests {
		c.Logf("test %d: %s", i, test.info)

		oldcfg := test.config(c)
		newcfg := s.config
		expected := gce.ConfigAttrs

		// In the case of immutable fields, the new config may need
		// to be updated to match the old config.
		fixes := test.attrFixes()
		if len(fixes) > 0 {
			updated, err := newcfg.Apply(fixes)
			c.Check(err, jc.ErrorIsNil)
			newcfg = updated
			expected = expected.Merge(fixes)
		}

		// Validate the new config (relative to the old one) using the
		// provider.
		validatedConfig, err := gce.Provider.Validate(newcfg, oldcfg)

		// Check the result
		if test.err != "" {
			test.checkFailure(c, err, "invalid base config")
		} else {
			c.Check(err, jc.ErrorIsNil)
			// We verify that Validate filled in the defaults
			// appropriately without
			test.checkAttrs(c, expected, validatedConfig)
		}
	}
}

var changeConfigTests = []configTestSpec{{
	info:   "no change, no error",
	expect: gce.ConfigAttrs,
}, {
	info:   "cannot change private-key",
	insert: testing.Attrs{"private-key": "okkult"},
	err:    "private-key: cannot change from seekrit to okkult",
}, {
	info:   "cannot change client-id",
	insert: testing.Attrs{"client-id": "mutant"},
	err:    "client-id: cannot change from static to mutant",
}, {
	info:   "cannot change client-email",
	insert: testing.Attrs{"client-email": "spam@eggs.com"},
	err:    "client-email: cannot change from joe@mail.com to spam@eggs.com",
}, {
	info:   "cannot change region",
	insert: testing.Attrs{"region": "not home"},
	err:    "region: cannot change from home to not home",
}, {
	info:   "cannot change project-id",
	insert: testing.Attrs{"project-id": "your-juju"},
	err:    "project-id: cannot change from my-juju to your-juju",
}, {
	info:   "can insert unknown field",
	insert: testing.Attrs{"unknown": "ignoti"},
	expect: testing.Attrs{"unknown": "ignoti"},
}}

// TODO(wwitzel3) refactor this to the provider_test file.
func (s *ConfigSuite) TestValidateChange(c *gc.C) {
	for i, test := range changeConfigTests {
		c.Logf("test %d: %s", i, test.info)

		testConfig := test.config(c)
		validatedConfig, err := gce.Provider.Validate(testConfig, s.config)

		// Check the result.
		if test.err != "" {
			test.checkFailure(c, err, "invalid config change")
		} else {
			test.checkSuccess(c, validatedConfig, err)
		}
	}
}

func (s *ConfigSuite) TestSetConfig(c *gc.C) {
	for i, test := range changeConfigTests {
		c.Logf("test %d: %s", i, test.info)

		environ, err := environs.New(s.config)
		c.Assert(err, jc.ErrorIsNil)

		testConfig := test.config(c)
		err = environ.SetConfig(testConfig)

		// Check the result.
		if test.err != "" {
			test.checkFailure(c, err, "invalid config change")
			test.checkAttrs(c, environ.Config().AllAttrs(), s.config)
		} else {
			test.checkSuccess(c, environ.Config(), err)
		}
	}
}
