package service

import "fahrradverleih/framework/domain"

// Die Repository-Interfaces definiert der Service, nicht der Store.
// So lässt sich die Datenhaltung austauschen, ohne die Anwendungsfälle zu ändern.

type ItemRepository interface {
	CreateItem(domain.Item) domain.Item
	Item(id int) (domain.Item, error)
	UpdateItem(domain.Item) error
	DeleteItem(id int) error
	ListItems() []domain.Item
}

type PersonRepository interface {
	CreatePerson(domain.Person) domain.Person
	Person(id int) (domain.Person, error)
	UpdatePerson(domain.Person) error
	DeletePerson(id int) error
	ListPersons() []domain.Person
}

type BookingRepository interface {
	CreateBooking(domain.Booking) domain.Booking
	Booking(id int) (domain.Booking, error)
	UpdateBooking(domain.Booking) error
	DeleteBooking(id int) error
	ListBookings() []domain.Booking
}

type Repository interface {
	ItemRepository
	PersonRepository
	BookingRepository
}
