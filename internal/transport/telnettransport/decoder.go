package telnettransport

const (
	iac  = 255
	sb   = 250
	se   = 240
	will = 251
	wont = 252
	do   = 253
	dont = 254
	echo = 1
	sga  = 3
)

type decoderState uint8

const (
	stateData decoderState = iota
	stateIAC
	stateOption
	stateSubnegotiation
	stateSubnegotiationIAC
)

// Decoder incrementally removes Telnet negotiation and produces negotiation replies.
type Decoder struct {
	state decoderState
	verb  byte
}

// Push consumes one arbitrary TCP fragment and preserves incomplete commands.
func (d *Decoder) Push(input []byte) (data, responses []byte) {
	for _, value := range input {
		switch d.state {
		case stateData:
			if value == iac {
				d.state = stateIAC
			} else {
				data = append(data, value)
			}
		case stateIAC:
			switch value {
			case iac:
				data = append(data, iac)
				d.state = stateData
			case sb:
				d.state = stateSubnegotiation
			case will, wont, do, dont:
				d.verb = value
				d.state = stateOption
			default:
				d.state = stateData
			}
		case stateOption:
			responses = append(responses, negotiate(d.verb, value)...)
			d.state = stateData
		case stateSubnegotiation:
			if value == iac {
				d.state = stateSubnegotiationIAC
			}
		case stateSubnegotiationIAC:
			if value == se {
				d.state = stateData
			} else {
				d.state = stateSubnegotiation
			}
		}
	}
	return data, responses
}

func negotiate(verb, option byte) []byte {
	switch verb {
	case will:
		if option == echo || option == sga {
			return []byte{iac, do, option}
		}
		return []byte{iac, dont, option}
	case do:
		if option == sga {
			return []byte{iac, will, option}
		}
		return []byte{iac, wont, option}
	default:
		return nil
	}
}
