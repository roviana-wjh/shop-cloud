package common

// Each process persists only its own maps in a separate file.
type State struct {
	Products           map[string]Product     `json:"products,omitempty"`
	Stock              map[string]int64       `json:"stock,omitempty"`
	Reservations       map[string]Reservation `json:"reservations,omitempty"`
	Orders             map[string]Order       `json:"orders,omitempty"`
	Payments           map[string]Payment     `json:"payments,omitempty"`
	ByOrder            map[string]string      `json:"byOrder,omitempty"`
	Sagas              map[string]Saga        `json:"sagas,omitempty"`
	ReservationByOrder map[string]string      `json:"reservationByOrder,omitempty"`
}
