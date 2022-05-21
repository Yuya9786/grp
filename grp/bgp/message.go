package bgp

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"time"
)

var (
	BGP_MARKER [16]byte = [16]byte{
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}
)

var (
	ErrInvalidMessageType error = errors.New("Invalid Message Type")
)

type Packet struct {
	Header  *Header
	Message Message
}

type Header struct {
	Maker  [16]byte
	Length uint16
	Type   MessageType
}

type MessageType uint8

const (
	OPEN         MessageType = 1
	UPDATE       MessageType = 2
	NOTIFICATION MessageType = 3
	KEEPALIVE    MessageType = 4
)

type Message interface {
	Type() MessageType
	Decode(l int) ([]byte, error)
}

type Open struct {
	Version    uint8
	AS         uint16
	HoldTime   uint16
	Identifier net.IP
	OptParmLen uint8
	Options    []*Option
}

type Option struct {
	Type   ParameterType
	Length uint8
	Value  []byte
}

type ParameterType uint8

const (
	AUTH_INFO ParameterType = 1
)

type Update struct {
	WithdrawnRoutesLen           uint16
	WithdrawnRoutes              []*Prefix
	TotalPathAttrLen             uint16
	PathAttrs                    []*PathAttr
	NetworkLayerReachabilityInfo []*Prefix
}

type Prefix struct {
	Length uint8
	Prefix net.IP
}

// Path attributes fall into four separate categories:
// 		1. Well-known mandatory.
// 		2. Well-known discretionary
// 		3. Optional transitive.
// 		4. Optional non-transitive.
// All well-known attributes must be recognized by all implementations.
type PathAttr struct {
	Flags uint8
	Type  PathAttrType
	Value []byte
}

const (
	PATH_ATTR_FLAG_OPTIONAL   uint8 = 1 << 7
	PATH_ATTR_FLAG_TRANSITIVE uint8 = 1 << 6
	PATH_ATTR_FLAG_PARTIAL    uint8 = 1 << 5
	PATH_ATTR_FLAG_EXTENDED   uint8 = 1 << 4
)

type PathAttrType uint8

const (
	ORIGIN           PathAttrType = 1 // Well-known mandatory attribute
	AS_PATH          PathAttrType = 2 // Well-known mandatory attribute
	NEXT_HOP         PathAttrType = 3 //
	MULTI_EXIT_DISC  PathAttrType = 4
	LOCAL_PREF       PathAttrType = 5 // Well-known discretionary attribute
	ATOMIC_AGGREGATE PathAttrType = 6 // Well-known discretionary attribute
	AGGREGATOR       PathAttrType = 7 // Optional transitive attribute
)

type NLRI []*Prefix

type KeepAlive struct{}

type Notification struct {
	ErrorCode *ErrorCode
	Data      []byte
}

type ErrorCode struct {
	Code    uint8
	Subcode uint8
}

const (
	MESSAGE_HEADER_ERROR       uint8 = 1
	OPEN_MESSAGE_ERROR         uint8 = 2
	UPDATE_MESSAGE_ERROR       uint8 = 3
	HOLD_TIMER_EXPIRED         uint8 = 4
	FINITE_STATE_MACHINE_ERROR uint8 = 5
	CEASE                      uint8 = 6
)

const (
	UNKNOWN_SUBCODE uint8 = 0
	// Message Header Error subcodes
	CONNECTION_NOT_SYNCHRONIZED uint8 = 1
	BAD_MESSAGE_LENGTH          uint8 = 2
	BAD_MESSAGE_TYPE            uint8 = 3
	// OPEN Message Error subcodes
	UNSUPPORTED_VERSION_NUMBER     uint8 = 1
	BAD_PEER_AS                    uint8 = 2
	BAD_BGP_IDENTIFIER             uint8 = 3
	UNSUPPORTED_OPTIONAL_PARAMETER uint8 = 4
	AUTHENTICATION_FAILURE         uint8 = 5
	UNACCEPTABLE_HOLD_TIME         uint8 = 6
	// UPDATE Message Error subcodes
	MALFORMED_ATTRIBUTE_LIST          uint8 = 1
	UNRECOGNIZED_WELL_KNOWN_ATTRIBUTE uint8 = 2
	MISSING_WELL_KNOWN_ATTRIBUTE      uint8 = 3
	ATTRIBUTE_FLAGS_ERROR             uint8 = 4
	ATTRIBUTE_LENGTH_ERROR            uint8 = 5
	INVALID_ORIGIN_ATTRIBUTE          uint8 = 6
	AS_ROUTING_LOOP                   uint8 = 7
	INVALID_NEXT_HOP_ATTRIBUTE        uint8 = 8
	OPTIONAL_ATTRIBUTE_ERROR          uint8 = 9
	INVALID_NETWORK_FIELD             uint8 = 10
	MALFORMED_AS_PATH                 uint8 = 11
)

func NewErrorCode(code, subcode uint8) *ErrorCode {
	if code == 0 || code > 6 {
		return nil
	}
	switch code {
	case MESSAGE_HEADER_ERROR:
		if subcode > 3 {
			return nil
		}
		return &ErrorCode{Code: code, Subcode: subcode}
	case OPEN_MESSAGE_ERROR:
		if subcode > 6 {
			return nil
		}
		return &ErrorCode{Code: code, Subcode: subcode}
	case UPDATE_MESSAGE_ERROR:
		if subcode > 11 {
			return nil
		}
		return &ErrorCode{Code: code, Subcode: subcode}
	default:
		return &ErrorCode{Code: code, Subcode: 0}
	}
}

func (e *ErrorCode) Error() string {
	switch e.Code {
	case MESSAGE_HEADER_ERROR:
		switch e.Subcode {
		case CONNECTION_NOT_SYNCHRONIZED:
			return "Message Header Error(Connection Not Synchronized)"
		case BAD_MESSAGE_LENGTH:
			return "Message Header Error(Bad Message Length)"
		case BAD_MESSAGE_TYPE:
			return "Message Header Error(Bad Message Type)"
		default:
			return "Message Header Error"
		}
	case OPEN_MESSAGE_ERROR:
		switch e.Subcode {
		case UNSUPPORTED_VERSION_NUMBER:
			return "OPEN Message Error(Unsupported Version Number)"
		case BAD_PEER_AS:
			return "OPEN Message Error(Bad Peer AS)"
		case BAD_BGP_IDENTIFIER:
			return "OPEN Message Error(Bad BGP Identifier)"
		default:
			return "OPEN Message Error"
		}
	case UPDATE_MESSAGE_ERROR:
		switch e.Subcode {
		case MALFORMED_ATTRIBUTE_LIST:
			return "UPDATE Message Error(Malformed Attribute List)"
		case UNRECOGNIZED_WELL_KNOWN_ATTRIBUTE:
			return "UPDATE Message Error(Unrecognized Well-known Attribute)"
		case MISSING_WELL_KNOWN_ATTRIBUTE:
			return "UPDATE Message Error(Missing Well-known Attribute)"
		case ATTRIBUTE_FLAGS_ERROR:
			return "UPDATE Message Error(Attribute Flags Error)"
		case ATTRIBUTE_LENGTH_ERROR:
			return "UPDATE Message Error(Attribute Length Error)"
		case INVALID_ORIGIN_ATTRIBUTE:
			return "UPDATE Message Error(Invalid ORIGIN Attribute)"
		case AS_ROUTING_LOOP:
			return "UPDATE Message Error(AS Routing Loop)"
		case INVALID_NEXT_HOP_ATTRIBUTE:
			return "UPDATE Message Error(Invalid NEXT_HOP Attribute)"
		case OPTIONAL_ATTRIBUTE_ERROR:
			return "UPDATE Message Error(Optional Attribute Error)"
		case INVALID_NETWORK_FIELD:
			return "UPDATE Message Error(Invalid Network Field)"
		case MALFORMED_AS_PATH:
			return "UPDATE Message Error(Malformed AS_PATH)"
		default:
			return "UPDATE Message Error"
		}
	case HOLD_TIMER_EXPIRED:
		return "Hold Timer Expired"
	case FINITE_STATE_MACHINE_ERROR:
		return "Finite State Machine Error"
	case CEASE:
		return "Cease"
	default:
		return "Unknown Error"
	}
}

func GetMessage[T *Open | *Update | *Notification | *KeepAlive](msg Message) T {
	return msg.(T)
}

func (*Open) Type() MessageType {
	return OPEN
}

func (*Update) Type() MessageType {
	return UPDATE
}

func (*Notification) Type() MessageType {
	return NOTIFICATION
}

func (*KeepAlive) Type() MessageType {
	return KEEPALIVE
}

func NewPacket(msgType MessageType) *Packet {
	return &Packet{
		Header: &Header{Maker: BGP_MARKER, Length: 19, Type: msgType},
	}
}

func Parse(data []byte) (*Packet, error) {
	buf := bytes.NewBuffer(data)
	packet := &Packet{Header: &Header{}}
	if err := binary.Read(buf, binary.BigEndian, packet.Header); err != nil {
		return nil, err
	}
	switch packet.Header.Type {
	case OPEN:
		op, err := ParseOpenMsg(buf.Bytes())
		if err != nil {
			return nil, err
		}
		packet.Message = op
	case UPDATE:
		upd, err := ParseUpdateMsg(buf.Bytes())
		if err != nil {
			return nil, err
		}
		packet.Message = upd
	case NOTIFICATION:
		notif, err := ParseNotificationMsg(buf.Bytes())
		if err != nil {
			return nil, err
		}
		packet.Message = notif
	case KEEPALIVE:
		packet.Message = &KeepAlive{}
	default:
		return nil, ErrInvalidMessageType
	}
	return packet, nil
}

func (p *Packet) Decode() ([]byte, error) {
	hdr, err := p.Header.Decode()
	if err != nil {
		return nil, err
	}
	msg, err := p.Message.Decode(int(p.Header.Length))
	if err != nil {
		return nil, err
	}
	return append(hdr, msg...), nil
}

func (h *Header) Decode() ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0, 19))
	if err := binary.Write(buf, binary.BigEndian, h.Maker); err != nil {
		return nil, fmt.Errorf("decode header marker: %w", err)
	}
	if err := binary.Write(buf, binary.BigEndian, h.Length); err != nil {
		return nil, fmt.Errorf("decode header length: %w", err)
	}
	if err := binary.Write(buf, binary.BigEndian, h.Type); err != nil {
		return nil, fmt.Errorf("decode header type: %w", err)
	}
	return buf.Bytes(), nil
}

func ParseOpenMsg(data []byte) (*Open, error) {
	type openNoOpt struct {
		Version    uint8
		As         uint16
		HoldTime   uint16
		Identifier uint32
		OptParmLen uint8
	}
	o := &openNoOpt{}
	buf := bytes.NewBuffer(data)
	if err := binary.Read(buf, binary.BigEndian, o); err != nil {
		return nil, err
	}
	options := make([]*Option, 0, o.OptParmLen)
	for buf.Len() > 0 {
		optType, err := buf.ReadByte()
		if err != nil {
			return nil, err
		}
		l, err := buf.ReadByte()
		if err != nil {
			return nil, err
		}
		options = append(options, &Option{
			Type:   ParameterType(optType),
			Length: l,
			Value:  buf.Next(int(l)),
		})
	}
	ip := make(net.IP, 4)
	binary.BigEndian.PutUint32(ip, o.Identifier)
	return &Open{
		Version:    o.Version,
		AS:         o.As,
		HoldTime:   o.HoldTime,
		Identifier: ip,
		OptParmLen: o.OptParmLen,
		Options:    options,
	}, nil
}

func ParseUpdateMsg(data []byte) (*Update, error) {
	buf := bytes.NewBuffer(data)
	update := &Update{}
	if err := binary.Read(buf, binary.BigEndian, &update.WithdrawnRoutesLen); err != nil {
		return nil, fmt.Errorf("parse update msg witdrawn routes len: %w", err)
	}
	wBuf := bytes.NewBuffer(buf.Next(int(update.WithdrawnRoutesLen)))
	wRoutes := make([]*Prefix, 0)
	for wBuf.Len() > 0 {
		l, err := wBuf.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("parse update msg withdrawn routes len: %w", err)
		}
		wRoutes = append(wRoutes, &Prefix{Length: l, Prefix: wBuf.Next(int(l))})
	}
	update.WithdrawnRoutes = wRoutes
	if err := binary.Read(buf, binary.BigEndian, &update.TotalPathAttrLen); err != nil {
		return nil, fmt.Errorf("parse update msg total path attrs len: %w", err)
	}
	pathAttrBuf := bytes.NewBuffer(buf.Next(int(update.TotalPathAttrLen)))
	pathAttrs := make([]*PathAttr, 0)
	for pathAttrBuf.Len() > 0 {
		flag, err := pathAttrBuf.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("parse update msg total path attr flag: %w", err)
		}
		pathAttrType, err := pathAttrBuf.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("parse update msg total path attr type: %w", err)
		}
		l, err := pathAttrBuf.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("parse update msg total path attr len: %w", err)
		}
		pathAttrs = append(pathAttrs, &PathAttr{
			Flags: flag,
			Type:  PathAttrType(pathAttrType),
			Value: pathAttrBuf.Next(int(l)),
		})
	}
	update.PathAttrs = pathAttrs
	nlri := make([]*Prefix, 0)
	for buf.Len() > 0 {
		pref, err := buf.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("parse update msg nlri len: %w", err)
		}
		l := pref / 8
		if pref%8 != 0 {
			l++
		}
		addr := make([]byte, l)
		if err := binary.Read(buf, binary.BigEndian, addr); err != nil {
			return nil, err
		}
		addr = append(addr, make([]byte, 4-l)...)
		nlri = append(nlri, &Prefix{
			Length: pref,
			Prefix: net.IP(addr),
		})
	}
	update.NetworkLayerReachabilityInfo = nlri
	return update, nil
}

func ParseNotificationMsg(data []byte) (*Notification, error) {
	buf := bytes.NewBuffer(data)
	notification := &Notification{ErrorCode: &ErrorCode{}}
	if err := binary.Read(buf, binary.BigEndian, notification.ErrorCode); err != nil {
		return nil, err
	}
	notification.Data = buf.Bytes()
	return notification, nil
}

func (o *Open) Decode(l int) ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0, l))
	if err := binary.Write(buf, binary.BigEndian, o.Version); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.BigEndian, o.AS); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.BigEndian, o.HoldTime); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.BigEndian, o.Identifier.To4()); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.BigEndian, o.OptParmLen); err != nil {
		return nil, err
	}
	for _, opt := range o.Options {
		if err := binary.Write(buf, binary.BigEndian, opt.Type); err != nil {
			return nil, err
		}
		if err := binary.Write(buf, binary.BigEndian, opt.Length); err != nil {
			return nil, err
		}
		if err := binary.Write(buf, binary.BigEndian, opt.Value); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

func (o *Open) Dump() string {
	str := ""
	return str
}

func (u *Update) Decode(l int) ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0, l))
	if err := binary.Write(buf, binary.BigEndian, u.WithdrawnRoutesLen); err != nil {
		return nil, err
	}
	for _, wr := range u.WithdrawnRoutes {
		if err := binary.Write(buf, binary.BigEndian, wr.Length); err != nil {
			return nil, err
		}
		length := wr.Length / 8
		if wr.Length%8 != 0 {
			length++
		}
		if err := binary.Write(buf, binary.BigEndian, wr.Prefix[:length]); err != nil {
			return nil, err
		}
	}
	if err := binary.Write(buf, binary.BigEndian, u.TotalPathAttrLen); err != nil {
		return nil, err
	}
	for _, attr := range u.PathAttrs {
		if err := binary.Write(buf, binary.BigEndian, attr.Flags); err != nil {
			return nil, err
		}
		if err := binary.Write(buf, binary.BigEndian, attr.Type); err != nil {
			return nil, err
		}
		if err := binary.Write(buf, binary.BigEndian, uint8(len(attr.Value))); err != nil {
			return nil, err
		}
		if err := binary.Write(buf, binary.BigEndian, attr.Value); err != nil {
			return nil, err
		}
	}
	for _, nlri := range u.NetworkLayerReachabilityInfo {
		if err := binary.Write(buf, binary.BigEndian, nlri.Length); err != nil {
			return nil, err
		}
		length := nlri.Length / 8
		if nlri.Length%8 != 0 {
			length++
		}
		if err := binary.Write(buf, binary.BigEndian, nlri.Prefix[:length]); err != nil {
			return nil, fmt.Errorf("decode update nlri prefix: %w", err)
		}
	}
	return buf.Bytes(), nil
}

func (u *Update) Dump() string {
	str := ""
	return str
}

func (*KeepAlive) Decode(l int) ([]byte, error) {
	return []byte{}, nil
}

func (*KeepAlive) Dump() string {
	return ""
}

func (n *Notification) Decode(l int) ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0, l))
	if err := binary.Write(buf, binary.BigEndian, n.ErrorCode.Code); err != nil {
		return nil, fmt.Errorf("decode notification error code: %w", err)
	}
	if err := binary.Write(buf, binary.BigEndian, n.ErrorCode.Subcode); err != nil {
		return nil, fmt.Errorf("decode notification error subcode: %w", err)
	}
	if err := binary.Write(buf, binary.BigEndian, n.Data); err != nil {
		return nil, fmt.Errorf("decode notification data: %w", err)
	}
	return buf.Bytes(), nil
}

func (n *Notification) Dump() string {
	str := ""
	return str
}

type messageBuilder struct {
	packet       *Packet
	typ          MessageType
	open         *Open
	update       *Update
	keepalive    *KeepAlive
	notification *Notification
}

func Builder(msgType MessageType) *messageBuilder {
	b := &messageBuilder{
		packet: &Packet{Header: &Header{Maker: BGP_MARKER, Length: 19, Type: msgType}},
		typ:    msgType,
	}
	switch msgType {
	case OPEN:
		b.open = &Open{
			Version:    4,
			AS:         0,
			HoldTime:   0,
			OptParmLen: 0,
			Identifier: nil,
			Options:    []*Option{},
		}
	case KEEPALIVE:
		b.keepalive = &KeepAlive{}
	case UPDATE:
		b.update = &Update{
			WithdrawnRoutesLen:           0,
			WithdrawnRoutes:              []*Prefix{},
			TotalPathAttrLen:             0,
			PathAttrs:                    []*PathAttr{},
			NetworkLayerReachabilityInfo: []*Prefix{},
		}
	case NOTIFICATION:
		b.notification = &Notification{
			ErrorCode: nil,
			Data:      nil,
		}
	}
	return b
}

func (b *messageBuilder) Packet() *Packet {
	switch b.typ {
	case OPEN:
		b.packet.Header.Length += 10 + uint16(b.open.OptParmLen)
		b.packet.Message = b.open
		return b.packet
	case KEEPALIVE:
		b.packet.Message = b.keepalive
		return b.packet
	case UPDATE:
		var a uint16 = 0
		b.packet.Header.Length += 4 + uint16(b.update.WithdrawnRoutesLen) + uint16(b.update.TotalPathAttrLen)
		for _, p := range b.update.NetworkLayerReachabilityInfo {
			a += uint16(p.Length/8) + 1
			if p.Length%8 != 0 {
				a += 1
			}
		}
		b.packet.Message = b.update
		b.packet.Header.Length += a
		return b.packet
	case NOTIFICATION:
		b.packet.Message = b.notification
		b.packet.Header.Length += 2
		b.packet.Header.Length += uint16(len(b.notification.Data))
		return b.packet
	default:
		return nil
	}
}

func (b *messageBuilder) Message() Message {
	switch b.typ {
	case OPEN:
		return b.open
	case KEEPALIVE:
		return b.keepalive
	case UPDATE:
		return b.update
	case NOTIFICATION:
		return b.notification
	default:
		return nil
	}
}

// open message
func (b *messageBuilder) AS(as int) {
	if b.typ == OPEN && as > 0 {
		b.open.AS = uint16(as)
	}
}

func (b *messageBuilder) HoldTime(hold time.Duration) {
	if b.typ == OPEN {
		b.open.HoldTime = uint16(hold / time.Second)
	}
}

func (b *messageBuilder) Identifier(ident net.IP) {
	if b.typ == OPEN {
		b.open.Identifier = ident
	}
}

func (b *messageBuilder) Options(opts []*Option) {
	if b.typ == OPEN {
		var a uint8 = 0
		for _, opt := range opts {
			a += opt.Length
			a += 2
		}
		b.open.Options = append(b.open.Options, opts...)
		b.open.OptParmLen += a
	}
}

// update message
func (b *messageBuilder) WithdrawnRoutes(routes []*Prefix) {
	if b.typ == UPDATE {
		var a uint16 = 0
		for _, route := range routes {
			l := uint16(route.Length / 8)
			if route.Length%8 != 0 {
				l++
			}
			a += l
		}
		b.update.WithdrawnRoutes = append(b.update.WithdrawnRoutes, routes...)
		b.update.WithdrawnRoutesLen += a
	}
}

func (b *messageBuilder) PathAttrs(attrs []*PathAttr) {
	if b.typ == UPDATE {
		var a uint16 = 0
		for _, attr := range attrs {
			a += uint16(3 + len(attr.Value))
		}
		b.update.TotalPathAttrLen += a
		b.update.PathAttrs = append(b.update.PathAttrs, attrs...)
	}
}

func (b *messageBuilder) NLRI(routes []*Prefix) {
	if b.typ == UPDATE {
		b.update.NetworkLayerReachabilityInfo = append(b.update.NetworkLayerReachabilityInfo, routes...)
	}
}

// notification message
func (b *messageBuilder) ErrorCode(code *ErrorCode) {
	if b.typ == NOTIFICATION {
		b.notification.ErrorCode = code
	}
}

func (b *messageBuilder) Data(data []byte) {
	if b.typ == NOTIFICATION {
		b.notification.Data = data
	}
}
