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
	"strings"

	caldav2 "code.vikunja.io/api/pkg/caldav"
	"github.com/samedi/caldav-go/data"
	"github.com/samedi/caldav-go/ixml"
)

// How caldav-go renders a property of a namespace it has no prefix for.
const unprefixedCalendarColor = `<calendar-color/>`

const emptyNotFoundPropstat = `<D:propstat><D:prop></D:prop><D:status>HTTP/1.1 404 Not Found</D:status></D:propstat>`

func requestsCalendarColor(body []byte) bool {
	var propfind struct {
		Prop struct {
			Tags []xml.Name `xml:",any"`
		} `xml:"DAV: prop"`
	}
	if err := xml.Unmarshal(body, &propfind); err != nil {
		return false
	}
	for _, tag := range propfind.Prop.Tags {
		if tag == calendarColorProp {
			return true
		}
	}
	return false
}

func (vcls *VikunjaCaldavProjectStorage) rememberCollectionColor(r *data.Resource, hexColor string) {
	color := caldav2.AppleColor(hexColor)
	if color == "" {
		return
	}
	if vcls.collectionColors == nil {
		vcls.collectionColors = map[string]string{}
	}
	vcls.collectionColors[ixml.EscapeText(r.Path)] = color
}

// addCalendarColors answers calendar-color in a PROPFIND multistatus rendered
// by caldav-go, which knows no such property and reports it as a 404 without
// its namespace. colors maps the XML-escaped href of a collection to its colour.
func addCalendarColors(multistatus string, colors map[string]string) string {
	const responseStart, responseEnd = "<D:response>", "</D:response>"

	responses := strings.Split(multistatus, responseStart)
	for i := 1; i < len(responses); i++ {
		response := responses[i]
		href, _, _ := strings.Cut(strings.TrimPrefix(response, "<D:href>"), "</D:href>")
		color, has := colors[href]
		end := strings.LastIndex(response, responseEnd)
		if !has || end < 0 {
			responses[i] = strings.Replace(response, unprefixedCalendarColor, calendarColorTag(""), 1)
			continue
		}

		response = response[:end] +
			"<D:propstat><D:prop>" + calendarColorTag(color) + "</D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat>" +
			response[end:]
		response = strings.Replace(response, unprefixedCalendarColor, "", 1)
		responses[i] = strings.Replace(response, emptyNotFoundPropstat, "", 1)
	}
	return strings.Join(responses, responseStart)
}

func calendarColorTag(color string) string {
	tag := `<calendar-color xmlns="` + calendarColorProp.Space + `"`
	if color == "" {
		return tag + "/>"
	}
	return tag + ">" + xmlEscape(color) + "</calendar-color>"
}
