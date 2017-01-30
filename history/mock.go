package history

type mock struct{}

func NewMock() interface {
	EventReader
	EventWriter
} {
	return mock{}
}

func (m mock) AllEvents() ([]Event, error) {
	return nil, nil
}

func (m mock) EventsForService(namespace, service string) ([]Event, error) {
	return nil, nil
}

func (m mock) LogEvent(namespace, service string, msg string) error {
	return nil
}
