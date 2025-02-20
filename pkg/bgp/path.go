package bgp

import (
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"
	"sort"
	"sync"
	"time"

	"github.com/vishvananda/netlink"
)

type Path struct {
	id              int
	info            *peerInfo
	routes          []netlink.Route
	as              int
	nextHop         net.IP
	origin          Origin
	asPath          ASPath
	med             int
	localPref       int
	nlri            *Prefix
	recognizedAttrs []PathAttr
	reason          BestPathSelectionReason
	best            bool
	link            netlink.Link
	local           bool
	timestamp       time.Time
}

func newPath(info *peerInfo, as int, nextHop net.IP, origin Origin, asPath ASPath, med int, nlri *Prefix, attrs []PathAttr, link netlink.Link) *Path {
	rand.Seed(time.Now().UnixNano())
	return &Path{
		id:              rand.Int(),
		info:            info,
		as:              as,
		nextHop:         nextHop,
		origin:          origin,
		asPath:          asPath,
		med:             med,
		localPref:       100,
		nlri:            nlri,
		recognizedAttrs: attrs,
		reason:          REASON_NOT_COMPARED,
		best:            false,
		link:            link,
		local:           false,
		timestamp:       time.Now(),
	}
}

func (p *Path) String() string {
	attrTypes := ""
	for _, attr := range p.recognizedAttrs {
		attrTypes += attr.Type().String() + ","
	}
	if attrTypes == "" {
		attrTypes = "None"
	} else {
		attrTypes = attrTypes[:len(attrTypes)-1]
	}
	return fmt.Sprintf("AS=%d NEXT HOP=%s NLRI=%s ATTRIBUTES=%s Best=%v ID=%d", p.as, p.nextHop, p.nlri, attrTypes, p.best, p.id)
}

type BestPathConfig struct {
	mutex *sync.RWMutex
}

type BestPathSelectionReason int

// 1. WEIGHT attribute(Cisco specific)
// 2. compare LOCAL_PREF
// 3. is local generated route
// 4. shortest AS_PATH attribute
// 5. minimum ORIGIN attribute (IGP < EGP < INCOMPLETE)
// 6. minimum MULTI_EXIT_DISC
// 7. choose EBGP over IBGP
// 8. prefer to minumum IGP metric route to next hop
// 9. oldest route received from EBGP
// 10. minumum BGP peer router id
// 11. minimum BGP peer IP address
const (
	REASON_WEIGHT_ATTR            BestPathSelectionReason = iota
	REASON_LOCAL_PREF_ATTR        BestPathSelectionReason = iota
	REASON_LOCAL_ORIGINATED       BestPathSelectionReason = iota
	REASON_AS_PATH_ATTR           BestPathSelectionReason = iota
	REASON_ORIGIN_ATTR            BestPathSelectionReason = iota
	REASON_MULTI_EXIT_DISC        BestPathSelectionReason = iota
	REASON_AS_NUMBER              BestPathSelectionReason = iota
	REASON_IGP_METRIC_TO_NEXT_HOP BestPathSelectionReason = iota
	REASON_OLDER_ROUTE            BestPathSelectionReason = iota
	REASON_BGP_PEER_ROUTER_ID     BestPathSelectionReason = iota
	REASON_BGP_PEER_IP_ADDR       BestPathSelectionReason = iota

	REASON_INVALID      BestPathSelectionReason = 253
	REASON_NOT_COMPARED BestPathSelectionReason = 254
	REASON_ONLY_PATH    BestPathSelectionReason = 255
)

func (b BestPathSelectionReason) String() string {
	switch b {
	case REASON_NOT_COMPARED:
		return "Not compared"
	case REASON_ONLY_PATH:
		return "Only path"
	case REASON_WEIGHT_ATTR:
		return "Weight"
	case REASON_LOCAL_PREF_ATTR:
		return "Local pref"
	case REASON_LOCAL_ORIGINATED:
		return "originated by local"
	case REASON_AS_PATH_ATTR:
		return "AS path"
	case REASON_ORIGIN_ATTR:
		return "Origin"
	case REASON_MULTI_EXIT_DISC:
		return "multi exit disc"
	case REASON_AS_NUMBER:
		return "AS number"
	case REASON_IGP_METRIC_TO_NEXT_HOP:
		return "Metric to next hop"
	case REASON_OLDER_ROUTE:
		return "Older route"
	case REASON_BGP_PEER_ROUTER_ID:
		return "BGP peer route id"
	case REASON_BGP_PEER_IP_ADDR:
		return "BGP peer IP address"
	case REASON_INVALID:
		return "Invalid"
	default:
		return fmt.Sprintf("Unknown reason %d", b)
	}
}

// https://www.cisco.com/c/ja_jp/support/docs/ip/border-gateway-protocol-bgp/13753-25.html
func sortPathes(pathes []*Path) ([]*Path, BestPathSelectionReason) {
	if len(pathes) == 0 {
		return nil, REASON_INVALID
	}
	reason := REASON_NOT_COMPARED
	if len(pathes) == 1 {
		reason = REASON_ONLY_PATH
		pathes[0].reason = REASON_ONLY_PATH
	}
	p := pathes
	sort.SliceStable(p, func(i, j int) bool {
		var path *Path
		for k, f := range compareFuncs {
			p1 := p[i]
			p2 := p[j]
			path = f(p1, p2)
			if path != nil {
				reason = BestPathSelectionReason(k)
				path.reason = reason
				return path == p1
			}
		}
		if path == nil {
			return false
		}
		return path == pathes[i]
	})
	reason = pathes[0].reason
	return p, reason
}

type compareFunc func(p1, p2 *Path) *Path

func compareWeight(p1, p2 *Path) *Path {
	return nil
}

func compareLocalPref(p1, p2 *Path) *Path {
	if p1.localPref > p2.localPref {
		return p1
	}
	if p1.localPref < p2.localPref {
		return p2
	}
	return nil
}

func compareLocalOriginated(p1, p2 *Path) *Path {
	if p1.local {
		return p1
	}
	if p2.local {
		return p2
	}
	return nil
}

func compareASPath(p1, p2 *Path) *Path {
	var l1, l2 int
	for _, seg := range p1.asPath.Segments {
		switch seg.Type {
		case SEG_TYPE_AS_SET:
			l1++
		case SEG_TYPE_AS_SEQUENCE:
			l1 += len(seg.AS2)
		}
	}
	for _, seg := range p2.asPath.Segments {
		switch seg.Type {
		case SEG_TYPE_AS_SET:
			l2++
		case SEG_TYPE_AS_SEQUENCE:
			l2 += len(seg.AS2)
		}
	}
	if l1 > l2 {
		return p2
	}
	if l1 < l2 {
		return p1
	}
	return nil
}

func compareOrigin(p1, p2 *Path) *Path {
	if p1.origin.value < p2.origin.value {
		return p1
	}
	if p1.origin.value > p2.origin.value {
		return p2
	}
	return nil
}

func compareMed(p1, p2 *Path) *Path {
	med1 := GetFromPathAttrs[*MultiExitDisc](p1.recognizedAttrs)
	med2 := GetFromPathAttrs[*MultiExitDisc](p2.recognizedAttrs)
	if med1 == nil || med2 == nil {
		return nil
	}
	seq1 := p1.asPath.GetSequence()
	seq2 := p2.asPath.GetSequence()
	if len(seq1) == 0 || len(seq2) == 0 {
		return nil
	}
	if seq1[0] != seq2[0] {
		return nil
	}
	if med1.discriminator < med2.discriminator {
		return p1
	}
	if med1.discriminator > med2.discriminator {
		return p2
	}
	return nil
}

func compareASNumber(p1, p2 *Path) *Path {
	if p1.info.isIBGP() != p2.info.isIBGP() {
		if p1.info.isIBGP() {
			return p2
		}
		if p2.info.isIBGP() {
			return p1
		}
	}
	return nil
}

func compareMtricToNextHop(p1, p2 *Path) *Path {
	// TODO: implement
	return nil
}
func compareOlderRoute(p1, p2 *Path) *Path {
	if p1.timestamp.Unix() > p2.timestamp.Unix() {
		return p2
	}
	if p1.timestamp.Unix() < p2.timestamp.Unix() {
		return p1
	}
	return nil
}

func comparePeerRouterId(p1, p2 *Path) *Path {
	id1 := binary.BigEndian.Uint32(p1.info.neighbor.routerId.To4())
	id2 := binary.BigEndian.Uint32(p2.info.neighbor.routerId.To4())
	if id1 > id2 {
		return p2
	}
	if id1 < id2 {
		return p1
	}
	return nil
}
func comparePeerAddress(p1, p2 *Path) *Path {
	addr1 := binary.BigEndian.Uint32(p1.info.neighbor.addr.To4())
	addr2 := binary.BigEndian.Uint32(p2.info.neighbor.addr.To4())
	if addr1 > addr2 {
		return p2
	}
	if addr1 < addr2 {
		return p1
	}
	return nil
}

var compareFuncs []compareFunc = []compareFunc{
	compareWeight,
	compareLocalPref,
	compareLocalOriginated,
	compareASPath,
	compareOrigin,
	compareMed,
	compareASNumber,
	compareMtricToNextHop,
	compareOlderRoute,
	comparePeerRouterId,
	comparePeerAddress,
}
