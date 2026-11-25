// Vikunja is a to-do list application to facilitate your life.
// Copyright 2018-present Vikunja and contributors. All rights reserved.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package caldav

import (
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"code.vikunja.io/api/pkg/db"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const colorProp = `<I:calendar-color xmlns:I="http://apple.com/ns/ical/"/>`

// caldav-go cleans the trailing slash off collection hrefs.
func collectionHref(projectID int64) string {
	return ProjectBasePath + "/" + strconv.FormatInt(projectID, 10)
}

func TestPropfindCalendarColor(t *testing.T) {
	const ok = "HTTP/1.1 200 OK"
	const notFound = "HTTP/1.1 404 Not Found"
	displayName := xml.Name{Space: "DAV:", Local: "displayname"}

	propfind := func(t *testing.T, path string, params echo.PathValues, depth, prefer string, props ...string) testMultistatus {
		t.Helper()

		rec := httptest.NewRecorder()
		req := httptest.NewRequest("PROPFIND", path, strings.NewReader(propfindWithProps(props...)))
		req.Header.Set("Depth", depth)
		if prefer != "" {
			req.Header.Set("Prefer", prefer)
		}
		c := echo.New().NewContext(req, rec)
		c.Set("userBasicAuth", caldavFilterUser)
		if params != nil {
			c.SetPathValues(params)
		}
		requireHTTPStatus(t, http.StatusMultiStatus, rec, ProjectHandler(c))
		return parseMultistatus(t, rec)
	}

	t.Run("a project with a colour answers it", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		setStoredColor(t, 36, "007AFF")

		statuses, values := propfind(t, projectPath(36), projectParams(36), "0", "", `<D:displayname/>`, colorProp).props(t, collectionHref(36))
		assert.Equal(t, map[xml.Name]string{displayName: ok, appleCalendarColor: ok}, statuses)
		assert.Equal(t, "#007AFFFF", values[appleCalendarColor])
	})

	t.Run("a project without a colour answers 404 for it", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)

		statuses, _ := propfind(t, projectPath(36), projectParams(36), "0", "", `<D:displayname/>`, colorProp).props(t, collectionHref(36))
		assert.Equal(t, map[xml.Name]string{displayName: ok, appleCalendarColor: notFound}, statuses)
	})

	t.Run("the colour alone", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		setStoredColor(t, 36, "007AFF")

		ms := propfind(t, projectPath(36), projectParams(36), "0", "", colorProp)
		statuses, values := ms.props(t, collectionHref(36))
		assert.Equal(t, map[xml.Name]string{appleCalendarColor: ok}, statuses)
		assert.Equal(t, "#007AFFFF", values[appleCalendarColor])
		require.Len(t, ms.Responses, 1)
		assert.Len(t, ms.Responses[0].Propstats, 1, "an empty 404 propstat is left behind")
	})

	t.Run("the tasks of a coloured project have none", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		setStoredColor(t, 36, "007AFF")

		ms := propfind(t, projectPath(36), projectParams(36), "1", "", `<D:getetag/>`, colorProp)
		statuses, _ := ms.props(t, collectionHref(36))
		assert.Equal(t, ok, statuses[appleCalendarColor])
		statuses, _ = ms.props(t, projectPath(36)+"uid-caldav-test.ics")
		assert.Equal(t, notFound, statuses[appleCalendarColor])
	})

	t.Run("the home set answers each project's own colour", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		setStoredColor(t, 36, "007AFF")

		ms := propfind(t, ProjectHomeSetPath, nil, "1", "", `<D:displayname/>`, colorProp)
		statuses, values := ms.props(t, collectionHref(36))
		assert.Equal(t, map[xml.Name]string{displayName: ok, appleCalendarColor: ok}, statuses)
		assert.Equal(t, "#007AFFFF", values[appleCalendarColor])
		statuses, _ = ms.props(t, collectionHref(38))
		assert.Equal(t, map[xml.Name]string{displayName: ok, appleCalendarColor: notFound}, statuses)
	})

	t.Run("a minimal response still has the colour", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		setStoredColor(t, 36, "007AFF")

		ms := propfind(t, ProjectHomeSetPath, nil, "1", "return=minimal", `<D:displayname/>`, colorProp)
		statuses, values := ms.props(t, collectionHref(36))
		assert.Equal(t, map[xml.Name]string{displayName: ok, appleCalendarColor: ok}, statuses)
		assert.Equal(t, "#007AFFFF", values[appleCalendarColor])
		statuses, _ = ms.props(t, collectionHref(38))
		assert.Equal(t, map[xml.Name]string{displayName: ok}, statuses)
	})

	t.Run("no colour unless asked for", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		setStoredColor(t, 36, "007AFF")

		statuses, _ := propfind(t, projectPath(36), projectParams(36), "0", "", `<D:displayname/>`).props(t, collectionHref(36))
		assert.Equal(t, map[xml.Name]string{displayName: ok}, statuses)
	})

	t.Run("a stored colour cannot inject markup", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		setStoredColor(t, 36, "<a/>ff")

		_, values := propfind(t, projectPath(36), projectParams(36), "0", "", colorProp).props(t, collectionHref(36))
		assert.Equal(t, "#affFF", values[appleCalendarColor])
	})

	t.Run("iOS reads back the colour it set", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)

		proppatch(t, caldavFilterUser, 36, iosSetColor)
		_, values := propfind(t, ProjectHomeSetPath, nil, "1", "", colorProp).props(t, collectionHref(36))
		assert.Equal(t, "#007AFFFF", values[appleCalendarColor])
	})
}
