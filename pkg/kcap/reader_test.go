// +build kcap

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

package kcap

import (
	"context"
	"github.com/rabbitstack/fibratus/pkg/config"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestRead(t *testing.T) {
	r, err := NewReader("_fixtures/cap1.kcap", &config.Config{})
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = r.RecoverSnapshotters()
	require.NoError(t, err)

	ctx := context.Background()

	kevtsc, errs := r.Read(ctx)
	i := 0
	for {
		select {
		case kevt := <-kevtsc:
			require.NotNil(t, kevt)
			require.True(t, kevt.Seq > 0)
			i++
			if i == 100 {
				return
			}
		case err := <-errs:
			t.Fatal(t, err)
		}
	}
}
