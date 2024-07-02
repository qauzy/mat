package constant

import (
	"github.com/qauzy/mat/component/geodata/router"
)

type RuleGeoSite interface {
	GetDomainMatcher() router.DomainMatcher
}

type RuleGeoIP interface {
	GetIPMatcher() *router.GeoIPMatcher
}

type RuleGroup interface {
	GetRecodeSize() int
}
