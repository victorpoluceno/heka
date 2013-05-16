/***** BEGIN LICENSE BLOCK *****
# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this file,
# You can obtain one at http://mozilla.org/MPL/2.0/.
#
# The Initial Developer of the Original Code is the Mozilla Foundation.
# Portions created by the Initial Developer are Copyright (C) 2012
# the Initial Developer. All Rights Reserved.
#
# Contributor(s):
#   Mike Trinkala (trink@mozilla.com)
#
# ***** END LICENSE BLOCK *****/

package pipeline

import (
	"github.com/mozilla-services/heka/message"
	"net/http"
	"net/url"
	"strings"
)

type NagiosOutputConfig struct {
	// URL to the Nagios cmd.cgi
	Url string `toml:"url"`
	// Nagios username
	Username string `toml:"username"`
	// Nagios password
	Password string `toml:"password"`
}

func (self *NagiosOutput) ConfigStruct() interface{} {
	return &NagiosOutputConfig{
		Url: "http://localhost/cgi-bin/cmd.cgi",
	}
}

type NagiosOutput struct {
	conf   *NagiosOutputConfig
	client *http.Client
}

func (self *NagiosOutput) Init(config interface{}) (err error) {
	self.conf = config.(*NagiosOutputConfig)
	self.client = new(http.Client)
	return
}

func (self *NagiosOutput) Run(or OutputRunner, h PluginHelper) (err error) {
	inChan := or.InChan()

	var (
		plc     *PipelineCapture
		pack    *PipelinePack
		msg     *message.Message
		payload string
	)

	for plc = range inChan {
		pack = plc.Pack
		msg = pack.Message
		payload = msg.GetPayload()
		pos := strings.IndexAny(payload, ":")
		state := "3" // UNKNOWN
		if pos != -1 {
			switch payload[:pos] {
			case "OK":
				state = "0"
			case "WARNING":
				state = "1"
			case "CRITICAL":
				state = "2"
			}
		}

		data := url.Values{
			"cmd_typ":          {"30"}, // PROCESS_SERVICE_CHECK_RESULT
			"cmd_mod":          {"2"},  // CMDMODE_COMMIT
			"host":             {msg.GetHostname()},
			"service":          {msg.GetLogger()},
			"plugin_state":     {state},
			"plugin_output":    {payload[pos+1:]},
			"performance_data": {""}}
		req, err := http.NewRequest("POST", self.conf.Url,
			strings.NewReader(data.Encode()))
		if err == nil {
			req.SetBasicAuth(self.conf.Username, self.conf.Password)
			if resp, err := self.client.Do(req); err == nil {
				resp.Body.Close()
			} else {
				or.LogError(err)
			}
		} else {
			or.LogError(err)
		}
		pack.Recycle()
	}
	return
}
