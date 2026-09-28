package main

// DeviceStatus is the common status shape used across antenna-switch
// profiles (the AT-14's JSON API, the AS-1289's query-string protocol) so
// the UI code never needs to know which device it's talking to.
type DeviceStatus struct {
	Active int      // 0-based index of the active antenna, -1 if none
	Names  []string // always exactly the profile's port count long
}
