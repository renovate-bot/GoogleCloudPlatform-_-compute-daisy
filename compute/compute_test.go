//  Copyright 2017 Google Inc. All Rights Reserved.
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package compute

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"testing"

	"github.com/kylelemons/godebug/pretty"
	computeAlpha "google.golang.org/api/compute/v0.alpha"
	computeBeta "google.golang.org/api/compute/v0.beta"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/googleapi"
)

var (
	testProject                    = "test-project"
	testZone                       = "test-zone"
	testRegion                     = "test-region"
	testDisk                       = "test-disk"
	testDisk2                      = "test-disk2"
	testResize               int64 = 128
	testForwardingRule             = "test-forwarding-rule"
	testFirewallRule               = "test-firewall-rule"
	testImage                      = "test-image"
	testImageAlpha                 = "test-image-alpha"
	testImageBeta                  = "test-image-beta"
	testMachineImage               = "test-machine-image"
	testInstance                   = "test-instance"
	testInstanceAlpha              = "test-instance-alpha"
	testInstanceBeta               = "test-instance-beta"
	testNetwork                    = "test-network"
	testSubnetwork                 = "test-subnetwork"
	testTargetInstance             = "test-target-instance"
	testTargetHTTPProxy            = "test-target-http-proxy"
	testURLMap                     = "test-url-map"
	testBackendService             = "test-backend-service"
	testHealthCheck                = "test-health-check"
	testNetworkEndpointGroup       = "test-network-endpoint-group"
)

func TestShouldRetryWithWait(t *testing.T) {
	tests := []struct {
		desc string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"non googleapi.Error", errors.New("foo"), false},
		{"400 error", &googleapi.Error{Code: 400}, false},
		{"429 error", &googleapi.Error{Code: 429}, true},
		{"500 error", &googleapi.Error{Code: 500}, true},
		{"connection reset", errors.New("read tcp 192.168.10.2:59590->74.125.135.95:443: read: connection reset by peer"), true},
		{"EOF", errors.New("unexpected EOF"), true},
	}

	for _, tt := range tests {
		if got := shouldRetryWithWait(nil, tt.err, 0); got != tt.want {
			t.Errorf("%s case: shouldRetryWithWait == %t, want %t", tt.desc, got, tt.want)
		}
	}
}

func TestCreates(t *testing.T) {
	var getURL, insertURL *string
	var getErr, insertErr, waitErr error
	var getResp interface{}
	svr, c, err := NewTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		url := r.URL.String()
		if r.Method == "POST" && url == *insertURL {
			if insertErr != nil {
				w.WriteHeader(400)
				fmt.Fprintln(w, insertErr)
				return
			}
			buf := new(bytes.Buffer)
			if _, err := buf.ReadFrom(r.Body); err != nil {
				t.Fatal(err)
			}
			fmt.Fprintln(w, `{}`)
		} else if r.Method == "GET" && url == *getURL {
			if getErr != nil {
				w.WriteHeader(400)
				fmt.Fprintln(w, getErr)
				return
			}
			body, _ := json.Marshal(getResp)
			fmt.Fprintln(w, string(body))
		} else {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintln(w, "URL and Method not recognized:", r.Method, url)
		}
	}))
	if err != nil {
		t.Fatal(err)
	}
	defer svr.Close()
	c.zoneOperationsWaitFn = func(_, _, _ string) error { return waitErr }
	c.regionOperationsWaitFn = func(_, _, _ string) error { return waitErr }
	c.globalOperationsWaitFn = func(_, _ string) error { return waitErr }

	tests := []struct {
		desc                       string
		getErr, insertErr, waitErr error
		shouldErr                  bool
	}{
		{"normal case", nil, nil, nil, false},
		{"get err case", errors.New("get err"), nil, nil, true},
		{"insert err case", nil, errors.New("insert err"), nil, true},
		{"wait err case", nil, nil, errors.New("wait err"), true},
	}

	d := &compute.Disk{Name: testDisk}
	fr := &compute.ForwardingRule{Name: testForwardingRule}
	fir := &compute.Firewall{Name: testFirewallRule}
	im := &compute.Image{Name: testImage}
	imAlpha := &computeAlpha.Image{Name: testImageAlpha}
	imBeta := &computeBeta.Image{Name: testImageBeta}
	mi := &compute.MachineImage{Name: testMachineImage, SourceInstance: testInstance}
	in := &compute.Instance{Name: testInstance}
	inAlpha := &computeAlpha.Instance{Name: testInstanceAlpha}
	inBeta := &computeBeta.Instance{Name: testInstanceBeta}
	n := &compute.Network{Name: testNetwork}
	sn := &compute.Subnetwork{Name: testSubnetwork}
	ti := &compute.TargetInstance{Name: testTargetInstance}
	hp := &compute.TargetHttpProxy{Name: testTargetHTTPProxy}
	um := &compute.UrlMap{Name: testURLMap}
	bs := &compute.BackendService{Name: testBackendService}
	hc := &compute.HealthCheck{Name: testHealthCheck}
	neg := &compute.NetworkEndpointGroup{Name: testNetworkEndpointGroup}
	creates := []struct {
		name              string
		do                func() error
		getURL, insertURL string
		getResp, resource interface{}
	}{
		{
			"disks",
			func() error { return c.CreateDisk(testProject, testZone, d) },
			fmt.Sprintf("/%s/zones/%s/disks/%s?alt=json&prettyPrint=false", testProject, testZone, testDisk),
			fmt.Sprintf("/%s/zones/%s/disks?alt=json&prettyPrint=false", testProject, testZone),
			&compute.Disk{Name: testDisk},
			d,
		},
		{
			"forwardingRules",
			func() error { return c.CreateForwardingRule(testProject, testRegion, fr) },
			fmt.Sprintf("/%s/regions/%s/forwardingRules/%s?alt=json&prettyPrint=false", testProject, testRegion, testForwardingRule),
			fmt.Sprintf("/%s/regions/%s/forwardingRules?alt=json&prettyPrint=false", testProject, testRegion),
			&compute.ForwardingRule{Name: testForwardingRule},
			fr,
		},
		{
			"FirewallRules",
			func() error { return c.CreateFirewallRule(testProject, fir) },
			fmt.Sprintf("/%s/global/firewalls/%s?alt=json&prettyPrint=false", testProject, testFirewallRule),
			fmt.Sprintf("/%s/global/firewalls?alt=json&prettyPrint=false", testProject),
			&compute.Firewall{Name: testFirewallRule},
			fir,
		},
		{
			"images",
			func() error { return c.CreateImage(testProject, im) },
			fmt.Sprintf("/%s/global/images/%s?alt=json&prettyPrint=false", testProject, testImage),
			fmt.Sprintf("/%s/global/images?alt=json&prettyPrint=false", testProject),
			&compute.Image{Name: testImage},
			im,
		},
		{
			"images",
			func() error { return c.CreateImageAlpha(testProject, imAlpha) },
			fmt.Sprintf("/%s/global/images/%s?alt=json&prettyPrint=false", testProject, testImageAlpha),
			fmt.Sprintf("/%s/global/images?alt=json&prettyPrint=false", testProject),
			&computeAlpha.Image{Name: testImageAlpha},
			imAlpha,
		},
		{
			"images",
			func() error { return c.CreateImageBeta(testProject, imBeta) },
			fmt.Sprintf("/%s/global/images/%s?alt=json&prettyPrint=false", testProject, testImageBeta),
			fmt.Sprintf("/%s/global/images?alt=json&prettyPrint=false", testProject),
			&computeBeta.Image{Name: testImageBeta},
			imBeta,
		},
		{
			"machineImages",
			func() error { return c.CreateMachineImage(testProject, mi) },
			fmt.Sprintf("/%s/global/machineImages/%s?alt=json&prettyPrint=false", testProject, testMachineImage),
			fmt.Sprintf("/%s/global/machineImages?alt=json&prettyPrint=false", testProject),
			&compute.MachineImage{Name: testMachineImage, SourceInstance: testInstance},
			mi,
		},
		{
			"instances",
			func() error { return c.CreateInstance(testProject, testZone, in) },
			fmt.Sprintf("/%s/zones/%s/instances/%s?alt=json&prettyPrint=false", testProject, testZone, testInstance),
			fmt.Sprintf("/%s/zones/%s/instances?alt=json&prettyPrint=false", testProject, testZone),
			&compute.Instance{Name: testInstance},
			in,
		},
		{
			"instancesAlpha",
			func() error { return c.CreateInstanceAlpha(testProject, testZone, inAlpha) },
			fmt.Sprintf("/%s/zones/%s/instances/%s?alt=json&prettyPrint=false", testProject, testZone, testInstanceAlpha),
			fmt.Sprintf("/%s/zones/%s/instances?alt=json&prettyPrint=false", testProject, testZone),
			&computeAlpha.Instance{Name: testInstanceAlpha},
			inAlpha,
		},
		{
			"instancesBeta",
			func() error { return c.CreateInstanceBeta(testProject, testZone, inBeta) },
			fmt.Sprintf("/%s/zones/%s/instances/%s?alt=json&prettyPrint=false", testProject, testZone, testInstanceBeta),
			fmt.Sprintf("/%s/zones/%s/instances?alt=json&prettyPrint=false", testProject, testZone),
			&computeBeta.Instance{Name: testInstanceBeta},
			inBeta,
		},
		{
			"networks",
			func() error { return c.CreateNetwork(testProject, n) },
			fmt.Sprintf("/%s/global/networks/%s?alt=json&prettyPrint=false", testProject, testNetwork),
			fmt.Sprintf("/%s/global/networks?alt=json&prettyPrint=false", testProject),
			&compute.Network{Name: testNetwork},
			n,
		},
		{
			"subnetworks",
			func() error { return c.CreateSubnetwork(testProject, testRegion, sn) },
			fmt.Sprintf("/%s/regions/%s/subnetworks/%s?alt=json&prettyPrint=false", testProject, testRegion, testSubnetwork),
			fmt.Sprintf("/%s/regions/%s/subnetworks?alt=json&prettyPrint=false", testProject, testRegion),
			&compute.Subnetwork{Name: testSubnetwork},
			sn,
		},
		{
			"targetInstances",
			func() error { return c.CreateTargetInstance(testProject, testZone, ti) },
			fmt.Sprintf("/%s/zones/%s/targetInstances/%s?alt=json&prettyPrint=false", testProject, testZone, testTargetInstance),
			fmt.Sprintf("/%s/zones/%s/targetInstances?alt=json&prettyPrint=false", testProject, testZone),
			&compute.TargetInstance{Name: testTargetInstance},
			ti,
		},
		{
			"regionTargetHttpProxies",
			func() error { return c.CreateRegionTargetHTTPProxy(testProject, testRegion, hp) },
			fmt.Sprintf("/%s/regions/%s/targetHttpProxies/%s?alt=json&prettyPrint=false", testProject, testRegion, testTargetHTTPProxy),
			fmt.Sprintf("/%s/regions/%s/targetHttpProxies?alt=json&prettyPrint=false", testProject, testRegion),
			&compute.TargetHttpProxy{Name: testTargetHTTPProxy},
			hp,
		},
		{
			"regionUrlMaps",
			func() error { return c.CreateRegionURLMap(testProject, testZone, um) },
			fmt.Sprintf("/%s/zones/%s/urlMaps/%s?alt=json&prettyPrint=false", testProject, testZone, testURLMap),
			fmt.Sprintf("/%s/zones/%s/urlMaps?alt=json&prettyPrint=false", testProject, testZone),
			&compute.UrlMap{Name: testURLMap},
			um,
		},
		{
			"regionBackendServices",
			func() error { return c.CreateRegionBackendService(testProject, testZone, bs) },
			fmt.Sprintf("/%s/zones/%s/backendServices/%s?alt=json&prettyPrint=false", testProject, testZone, testBackendService),
			fmt.Sprintf("/%s/zones/%s/backendServices?alt=json&prettyPrint=false", testProject, testZone),
			&compute.BackendService{Name: testBackendService},
			bs,
		},
		{
			"regionHealthChecks",
			func() error { return c.CreateRegionHealthCheck(testProject, testZone, hc) },
			fmt.Sprintf("/%s/zones/%s/healthChecks/%s?alt=json&prettyPrint=false", testProject, testZone, testHealthCheck),
			fmt.Sprintf("/%s/zones/%s/healthChecks?alt=json&prettyPrint=false", testProject, testZone),
			&compute.HealthCheck{Name: testHealthCheck},
			hc,
		},
		{
			"regionNetworkEndpointGroups",
			func() error { return c.CreateRegionNetworkEndpointGroup(testProject, testRegion, neg) },
			fmt.Sprintf("/%s/regions/%s/networkEndpointGroups/%s?alt=json&prettyPrint=false", testProject, testRegion, testNetworkEndpointGroup),
			fmt.Sprintf("/%s/regions/%s/networkEndpointGroups?alt=json&prettyPrint=false", testProject, testRegion),
			&compute.NetworkEndpointGroup{Name: testNetworkEndpointGroup},
			neg,
		},
	}

	for _, create := range creates {
		getURL = &create.getURL
		insertURL = &create.insertURL
		getResp = create.getResp
		for _, tt := range tests {
			getErr, insertErr, waitErr = tt.getErr, tt.insertErr, tt.waitErr
			create.do()

			// We have to fudge this part in order to check that the returned resource == getResp.
			f := reflect.ValueOf(create.resource).Elem().FieldByName("ServerResponse")
			f.Set(reflect.Zero(f.Type()))

			if err != nil && !tt.shouldErr {
				t.Errorf("%s: got unexpected error: %s", tt.desc, err)
			} else if diff := pretty.Compare(create.resource, getResp); err == nil && diff != "" {
				t.Errorf("%s: Resource does not match expectation: (-got +want)\n%s", tt.desc, diff)
			}
		}
	}
}

func TestStarts(t *testing.T) {
	var startURL, opGetURL string
	svr, c, err := NewTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.String() == startURL {
			fmt.Fprint(w, `{}`)
		} else if r.Method == "POST" && r.URL.String() == opGetURL {
			fmt.Fprint(w, `{"Status":"DONE"}`)
		} else {
			w.WriteHeader(500)
			fmt.Fprintln(w, "URL and Method not recognized:", r.Method, r.URL)
		}
	}))
	if err != nil {
		t.Fatal(err)
	}
	defer svr.Close()

	startURL = fmt.Sprintf("/projects/%s/zones/%s/instances/%s/start?alt=json&prettyPrint=false", testProject, testZone, testInstance)
	opGetURL = fmt.Sprintf("/projects/%s/zones/%s/operations//wait?alt=json&prettyPrint=false", testProject, testZone)
	if err := c.StartInstance(testProject, testZone, testInstance); err != nil {
		t.Errorf("error running Start: %v", err)
	}
}

func TestStops(t *testing.T) {
	var stopURL, opGetURL string
	svr, c, err := NewTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.String() == stopURL {
			fmt.Fprint(w, `{}`)
		} else if r.Method == "POST" && r.URL.String() == opGetURL {
			fmt.Fprint(w, `{"Status":"DONE"}`)
		} else {
			w.WriteHeader(500)
			fmt.Fprintln(w, "URL and Method not recognized:", r.Method, r.URL)
		}
	}))
	if err != nil {
		t.Fatal(err)
	}
	defer svr.Close()

	stopURL = fmt.Sprintf("/projects/%s/zones/%s/instances/%s/stop?alt=json&prettyPrint=false", testProject, testZone, testInstance)
	opGetURL = fmt.Sprintf("/projects/%s/zones/%s/operations//wait?alt=json&prettyPrint=false", testProject, testZone)
	if err := c.StopInstance(testProject, testZone, testInstance); err != nil {
		t.Errorf("error running Stop: %v", err)
	}
}

func TestDeletes(t *testing.T) {
	var deleteURL, opGetURL *string
	svr, c, err := NewTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" && r.URL.String() == *deleteURL {
			fmt.Fprint(w, `{}`)
		} else if r.Method == "POST" && r.URL.String() == *opGetURL {
			fmt.Fprint(w, `{"Status":"DONE"}`)
		} else {
			w.WriteHeader(500)
			fmt.Fprintln(w, "URL and Method not recognized:", r.Method, r.URL)
		}
	}))
	if err != nil {
		t.Fatal(err)
	}
	defer svr.Close()

	deletes := []struct {
		name                string
		do                  func() error
		deleteURL, opGetURL string
	}{
		{
			"disks",
			func() error { return c.DeleteDisk(testProject, testZone, testDisk) },
			fmt.Sprintf("/projects/%s/zones/%s/disks/%s?alt=json&prettyPrint=false", testProject, testZone, testDisk),
			fmt.Sprintf("/projects/%s/zones/%s/operations//wait?alt=json&prettyPrint=false", testProject, testZone),
		},
		{
			"forwardingRules",
			func() error { return c.DeleteForwardingRule(testProject, testRegion, testForwardingRule) },
			fmt.Sprintf("/projects/%s/regions/%s/forwardingRules/%s?alt=json&prettyPrint=false", testProject, testRegion, testForwardingRule),
			fmt.Sprintf("/projects/%s/regions/%s/operations//wait?alt=json&prettyPrint=false", testProject, testRegion),
		},
		{
			"FirewallRules",
			func() error { return c.DeleteFirewallRule(testProject, testFirewallRule) },
			fmt.Sprintf("/projects/%s/global/firewalls/%s?alt=json&prettyPrint=false", testProject, testFirewallRule),
			fmt.Sprintf("/projects/%s/global/operations//wait?alt=json&prettyPrint=false", testProject),
		},
		{
			"images",
			func() error { return c.DeleteImage(testProject, testImage) },
			fmt.Sprintf("/projects/%s/global/images/%s?alt=json&prettyPrint=false", testProject, testImage),
			fmt.Sprintf("/projects/%s/global/operations//wait?alt=json&prettyPrint=false", testProject),
		},
		{
			"machineImages",
			func() error { return c.DeleteMachineImage(testProject, testMachineImage) },
			fmt.Sprintf("/projects/%s/global/machineImages/%s?alt=json&prettyPrint=false", testProject, testMachineImage),
			fmt.Sprintf("/projects/%s/global/operations//wait?alt=json&prettyPrint=false", testProject),
		},
		{
			"instances",
			func() error { return c.DeleteInstance(testProject, testZone, testInstance) },
			fmt.Sprintf("/projects/%s/zones/%s/instances/%s?alt=json&prettyPrint=false", testProject, testZone, testInstance),
			fmt.Sprintf("/projects/%s/zones/%s/operations//wait?alt=json&prettyPrint=false", testProject, testZone),
		},
		{
			"networks",
			func() error { return c.DeleteNetwork(testProject, testNetwork) },
			fmt.Sprintf("/projects/%s/global/networks/%s?alt=json&prettyPrint=false", testProject, testNetwork),
			fmt.Sprintf("/projects/%s/global/operations//wait?alt=json&prettyPrint=false", testProject),
		},
		{
			"subnetworks",
			func() error { return c.DeleteSubnetwork(testProject, testRegion, testSubnetwork) },
			fmt.Sprintf("/projects/%s/regions/%s/subnetworks/%s?alt=json&prettyPrint=false", testProject, testRegion, testSubnetwork),
			fmt.Sprintf("/projects/%s/regions/%s/operations//wait?alt=json&prettyPrint=false", testProject, testRegion),
		},
		{
			"targetInstances",
			func() error { return c.DeleteTargetInstance(testProject, testZone, testTargetInstance) },
			fmt.Sprintf("/projects/%s/zones/%s/targetInstances/%s?alt=json&prettyPrint=false", testProject, testZone, testTargetInstance),
			fmt.Sprintf("/projects/%s/zones/%s/operations//wait?alt=json&prettyPrint=false", testProject, testZone),
		},
		{
			"regionTargetHttpProxies",
			func() error { return c.DeleteRegionTargetHTTPProxy(testProject, testRegion, testTargetHTTPProxy) },
			fmt.Sprintf("/projects/%s/regions/%s/targetHttpProxies/%s?alt=json&prettyPrint=false", testProject, testRegion, testTargetHTTPProxy),
			fmt.Sprintf("/projects/%s/regions/%s/operations//wait?alt=json&prettyPrint=false", testProject, testRegion),
		},
		{
			"regionUrlMaps",
			func() error { return c.DeleteRegionURLMap(testProject, testRegion, testURLMap) },
			fmt.Sprintf("/projects/%s/regions/%s/urlMaps/%s?alt=json&prettyPrint=false", testProject, testRegion, testURLMap),
			fmt.Sprintf("/projects/%s/regions/%s/operations//wait?alt=json&prettyPrint=false", testProject, testRegion),
		},
		{
			"regionBackendServices",
			func() error { return c.DeleteRegionBackendService(testProject, testRegion, testBackendService) },
			fmt.Sprintf("/projects/%s/regions/%s/backendServices/%s?alt=json&prettyPrint=false", testProject, testRegion, testBackendService),
			fmt.Sprintf("/projects/%s/regions/%s/operations//wait?alt=json&prettyPrint=false", testProject, testRegion),
		},
		{
			"regionHealthChecks",
			func() error { return c.DeleteRegionHealthCheck(testProject, testRegion, testHealthCheck) },
			fmt.Sprintf("/projects/%s/regions/%s/healthChecks/%s?alt=json&prettyPrint=false", testProject, testRegion, testHealthCheck),
			fmt.Sprintf("/projects/%s/regions/%s/operations//wait?alt=json&prettyPrint=false", testProject, testRegion),
		},
		{
			"regionNetworkEndpointGroups",
			func() error {
				return c.DeleteRegionNetworkEndpointGroup(testProject, testRegion, testNetworkEndpointGroup)
			},
			fmt.Sprintf("/projects/%s/regions/%s/networkEndpointGroups/%s?alt=json&prettyPrint=false", testProject, testRegion, testNetworkEndpointGroup),
			fmt.Sprintf("/projects/%s/regions/%s/operations//wait?alt=json&prettyPrint=false", testProject, testRegion),
		},
	}

	for _, d := range deletes {
		deleteURL = &d.deleteURL
		opGetURL = &d.opGetURL
		if err := d.do(); err != nil {
			t.Errorf("%s: error running Delete: %v", d.name, err)
		}
	}
}

func TestDeprecateImage(t *testing.T) {
	svr, c, err := NewTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.String() == fmt.Sprintf("/projects/%s/global/images/%s/deprecate?alt=json&prettyPrint=false", testProject, testImage) {
			fmt.Fprint(w, `{}`)
		} else if r.Method == "POST" && r.URL.String() == fmt.Sprintf("/projects/%s/global/operations//wait?alt=json&prettyPrint=false", testProject) {
			fmt.Fprint(w, `{"Status":"DONE"}`)
		} else {
			w.WriteHeader(500)
			fmt.Fprintln(w, "URL and Method not recognized:", r.Method, r.URL)
		}
	}))
	if err != nil {
		t.Fatal(err)
	}
	defer svr.Close()

	if err := c.DeprecateImage(testProject, testImage, &compute.DeprecationStatus{}); err != nil {
		t.Fatalf("error running DeprecateImage: %v", err)
	}
}

func TestDeprecateImageAlpha(t *testing.T) {
	svr, c, err := NewTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.String() == fmt.Sprintf("/projects/%s/global/images/%s/deprecate?alt=json&prettyPrint=false", testProject, testImageAlpha) {
			fmt.Fprint(w, `{}`)
		} else if r.Method == "POST" && r.URL.String() == fmt.Sprintf("/projects/%s/global/operations//wait?alt=json&prettyPrint=false", testProject) {
			fmt.Fprint(w, `{"Status":"DONE"}`)
		} else {
			w.WriteHeader(500)
			fmt.Fprintln(w, "URL and Method not recognized:", r.Method, r.URL)
		}
	}))
	if err != nil {
		t.Fatal(err)
	}
	defer svr.Close()

	if err := c.DeprecateImageAlpha(testProject, testImageAlpha, &computeAlpha.DeprecationStatus{}); err != nil {
		t.Fatalf("error running DeprecateImageAlpha: %v", err)
	}
}
func TestAttachDisk(t *testing.T) {
	svr, c, err := NewTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.String() == fmt.Sprintf("/projects/%s/zones/%s/instances/%s/attachDisk?alt=json&prettyPrint=false", testProject, testZone, testInstance) {
			fmt.Fprint(w, `{}`)
		} else if r.Method == "POST" && r.URL.String() == fmt.Sprintf("/projects/%s/zones/%s/operations//wait?alt=json&prettyPrint=false", testProject, testZone) {
			fmt.Fprint(w, `{"Status":"DONE"}`)
		} else {
			w.WriteHeader(500)
			fmt.Fprintln(w, "URL and Method not recognized:", r.Method, r.URL)
		}
	}))
	if err != nil {
		t.Fatal(err)
	}
	defer svr.Close()

	if err := c.AttachDisk(testProject, testZone, testInstance, &compute.AttachedDisk{}); err != nil {
		t.Fatalf("error running AttachDisk: %v", err)
	}
}

func TestDetachDisk(t *testing.T) {
	svr, c, err := NewTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.String() == fmt.Sprintf("/projects/%s/zones/%s/instances/%s/detachDisk?alt=json&deviceName=%s&prettyPrint=false", testProject, testZone, testInstance, testDisk) {
			fmt.Fprint(w, `{}`)
		} else if r.Method == "POST" && r.URL.String() == fmt.Sprintf("/projects/%s/zones/%s/operations//wait?alt=json&prettyPrint=false", testProject, testZone) {
			fmt.Fprint(w, `{"Status":"DONE"}`)
		} else {
			w.WriteHeader(500)
			fmt.Fprintln(w, "URL and Method not recognized:", r.Method, r.URL)
		}
	}))
	if err != nil {
		t.Fatal(err)
	}
	defer svr.Close()

	if err := c.DetachDisk(testProject, testZone, testInstance, testDisk); err != nil {
		t.Fatalf("error running DetachDisk: %v", err)
	}
}

func TestSuspendResume(t *testing.T) {
	svr, c, err := NewTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.String() == fmt.Sprintf("/projects/%s/zones/%s/instances/%s/suspend?alt=json&prettyPrint=false", testProject, testZone, testInstance) {
			fmt.Fprint(w, `{}`)
		} else if r.Method == "POST" && r.URL.String() == fmt.Sprintf("/projects/%s/zones/%s/operations//wait?alt=json&prettyPrint=false", testProject, testZone) {
			fmt.Fprint(w, `{"Status":"DONE"}`)
		} else if r.Method == "POST" && r.URL.String() == fmt.Sprintf("/projects/%s/zones/%s/instances/%s/resume?alt=json&prettyPrint=false", testProject, testZone, testInstance) {
			fmt.Fprint(w, `{}`)
		} else {
			w.WriteHeader(500)
			fmt.Fprintln(w, "URL and Method not recognized:", r.Method, r.URL)
		}
	}))
	if err != nil {
		t.Fatal(err)
	}
	defer svr.Close()

	if err := c.Suspend(testProject, testZone, testInstance); err != nil {
		t.Fatalf("error running Suspend: %v", err)
	}
	if err := c.Resume(testProject, testZone, testInstance); err != nil {
		t.Fatalf("error running Resume: %v", err)
	}
}
