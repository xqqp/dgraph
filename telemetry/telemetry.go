/*
 * SPDX-FileCopyrightText: © Hypermode Inc. <hello@hypermode.com>
 * SPDX-License-Identifier: Apache-2.0
 */

package telemetry

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"runtime"
	"time"

	"github.com/golang/glog"
	"github.com/pkg/errors"

	"github.com/hypermodeinc/dgraph/v24/protos/pb"
	"github.com/hypermodeinc/dgraph/v24/worker"
	"github.com/hypermodeinc/dgraph/v24/x"
)

// Telemetry holds information about the state of the zero and alpha server.
type Telemetry struct {
	Arch           string   `json:",omitempty"`
	Cid            string   `json:",omitempty"`
	ClusterSize    int      `json:",omitempty"`
	DiskUsageMB    int64    `json:",omitempty"`
	NumAlphas      int      `json:",omitempty"`
	NumGroups      int      `json:",omitempty"`
	NumTablets     int      `json:",omitempty"`
	NumZeros       int      `json:",omitempty"`
	OS             string   `json:",omitempty"`
	SinceHours     int      `json:",omitempty"`
	Version        string   `json:",omitempty"`
	NumDQL         uint64   `json:",omitempty"`
	NumGraphQL     uint64   `json:",omitempty"`
	EEFeaturesList []string `json:",omitempty"`
	Codename       string   `json:",omitempty"`
}

const url = "https://ping.dgraph.io/3.0/projects/5b809dfac9e77c0001783ad0/events"

// NewZero returns a Telemetry struct that holds information about the state of zero server.
func NewZero(ms *pb.MembershipState) *Telemetry {
	if len(ms.GetCid()) == 0 {
		glog.V(2).Infoln("No CID found yet")
		return nil
	}
	t := &Telemetry{
		Cid:       ms.GetCid(),
		NumGroups: len(ms.GetGroups()),
		NumZeros:  len(ms.GetZeros()),
		Version:   x.Version(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		Codename:  x.Codename(),
	}
	for _, g := range ms.GetGroups() {
		t.NumAlphas += len(g.GetMembers())
		for _, tablet := range g.GetTablets() {
			t.NumTablets++
			t.DiskUsageMB += tablet.GetOnDiskBytes()
		}
	}
	t.DiskUsageMB /= (1 << 20)
	t.ClusterSize = t.NumAlphas + t.NumZeros
	return t
}

// NewAlpha returns a Telemetry struct that holds information about the state of alpha server.
func NewAlpha(ms *pb.MembershipState) *Telemetry {
	return &Telemetry{
		Cid:            ms.GetCid(),
		Version:        x.Version(),
		OS:             runtime.GOOS,
		Arch:           runtime.GOARCH,
		EEFeaturesList: worker.GetFeaturesList(),
		Codename:       x.Codename(),
	}
}

// Post reports the Telemetry to the stats server.
func (t *Telemetry) Post() error {
	data, err := json.Marshal(t)
	if err != nil {
		return err
	}

	var requestURL string
	if t.Version != "dev" {
		requestURL = url + "/pings"
	} else {
		requestURL = url + "/dev"
	}
	req, err := http.NewRequest("POST", requestURL, bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "D0398E8C83BB30F67C519FDA6175975F680921890C35B36C34BE1095445"+
		"97497CA758881BD7D56CC2355A2F36B4560102CBC3279AC7B27E5391372C36A31167EB0D06BF3764894AD20"+
		"A0554BAFF14C292A40BC252BB9FF008736A0FD1D44E085")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			glog.Warningf("error closing body: %v", err)
		}
	}()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != 201 {
		return errors.Errorf(string(body))
	}
	glog.V(2).Infof("Telemetry response status: %v", resp.Status)
	glog.V(2).Infof("Telemetry response body: %s", body)
	return nil
}
