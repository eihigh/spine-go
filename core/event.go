package spine

// EventData stores the setup pose values for an Event.
type EventData struct {
	// Name is unique across all events in the skeleton.
	Name        string
	Int         int
	Float       float32
	String      string
	AudioPath   string
	Volume      float32
	Balance     float32
}

// NewEventData creates event setup pose data.
func NewEventData(name string) *EventData {
	if name == "" {
		panic("spine: name cannot be empty")
	}
	return &EventData{Name: name}
}

// Event stores the current pose values for an event.
type Event struct {
	Data    *EventData
	Int     int
	Float   float32
	String  string
	Volume  float32
	Balance float32

	// Time is the animation time this event was keyed.
	Time float32
}

// NewEvent creates an event.
func NewEvent(time float32, data *EventData) *Event {
	if data == nil {
		panic("spine: data cannot be nil")
	}
	return &Event{Time: time, Data: data}
}
