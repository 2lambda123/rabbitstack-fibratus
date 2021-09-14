/*
 * Copyright 2019-2020 by Nedim Sabic Sabic
 * https://www.fibratus.io
 * All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package config

import "os"

type Config struct {
	BaseConfig
}

func NewWithOpts(options ...Option) *Config {
	config := newWithOpts(options...)

	config.addCommonFlags()
	config.addFlags()

	return config
}

func (c *Config) Init() error {
	return c.init()
}

func (c *Config) addFlags() {
	c.flags.String(configFile, "/etc/fibratus/fibratus.yml", "Indicates the location of the configuration file")
	if c.opts.run || c.opts.capture {
		c.flags.Int(watermark, 128, "")
		c.flags.Int(ringBufferSize, 8*os.Getpagesize(), "")
	}
}
