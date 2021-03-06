// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2018 Canonical Ltd
// Copyright (C) 2018 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package device

import (
	"fmt"

	"github.com/edgexfoundry/device-simple/internal/cache"
	"github.com/edgexfoundry/device-simple/internal/common"
	"github.com/edgexfoundry/device-simple/internal/transformer"
	ds_models "github.com/edgexfoundry/device-simple/pkg/models"
	"github.com/edgexfoundry/edgex-go/pkg/models"
)

// processAsyncResults processes readings that are pushed from
// a DS implementation. Each is reading is optionally transformed
// before being pushed to Core Data.
func processAsyncResults() {
	for !svc.stopped {
		acv := <-svc.asyncCh
		readings := make([]models.Reading, 0, len(acv.CommandValues))

		device, ok := cache.Devices().ForName(acv.DeviceName)
		if !ok {
			common.LoggingClient.Error(fmt.Sprintf("processAsyncResults - recieved Device %s not found in cache", acv.DeviceName))
			continue
		}

		for _, cv := range acv.CommandValues {
			// get the device resource associated with the rsp.RO
			do, ok := cache.Profiles().DeviceObject(device.Profile.Name, cv.RO.Object)
			if !ok {
				common.LoggingClient.Error(fmt.Sprintf("processAsyncResults - Device Resource %s not found in Device %s", cv.RO.Object, acv.DeviceName))
				continue
			}

			if common.CurrentConfig.Device.DataTransform {
				err := transformer.TransformReadResult(cv, do.Properties.Value)
				if err != nil {
					common.LoggingClient.Error(fmt.Sprintf("processAsyncResults - CommandValue (%s) transformed failed: %v", cv.String(), err))
					cv = ds_models.NewStringValue(cv.RO, cv.Origin, fmt.Sprintf("Transformation failed for device resource, with value: %s, property value: %v, and error: %v", cv.String(), do.Properties.Value, err))
				}
			}

			err := transformer.CheckAssertion(cv, do.Properties.Value.Assertion, &device)
			if err != nil {
				common.LoggingClient.Error(fmt.Sprintf("processAsyncResults - Assertion failed for device resource: %s, with value: %s and assertion: %s, %v", cv.RO.Object, cv.String(), do.Properties.Value.Assertion, err))
				cv = ds_models.NewStringValue(cv.RO, cv.Origin, fmt.Sprintf("Assertion failed for device resource, with value: %s and assertion: %s", cv.String(), do.Properties.Value.Assertion))
			}

			if len(cv.RO.Mappings) > 0 {
				newCV, ok := transformer.MapCommandValue(cv)
				if ok {
					cv = newCV
				} else {
					common.LoggingClient.Warn(fmt.Sprintf("processAsyncResults - Mapping failed for Device Resource Operation: %v, with value: %s, %v", cv.RO, cv.String(), err))
				}
			}

			reading := common.CommandValueToReading(cv, device.Name)
			readings = append(readings, *reading)
		}

		// push to Core Data
		event := &models.Event{Device: acv.DeviceName, Readings: readings}
		_, err := common.EventClient.Add(event)
		if err != nil {
			common.LoggingClient.Error(fmt.Sprintf("processAsyncResults - Failed to push event %v: %v", event, err))
		}
	}
}
