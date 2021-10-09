ivy
===

Ivy is an interpreter for an APL-like language. It is a plaything and a work in
progress.

Ivy has a custom domain. Do not install using github directly. Instead, run:

	go install robpike.io/ivy@latest

Documentation at https://pkg.go.dev/robpike.io/ivy.

Try the demo: within ivy, type

	)demo

Prototype apps for iPhone, iPad, and Android are available in the App store and Google Play store.
To find them, search for "ivy bignum calculator". (They are running a very old version of ivy.)

Slides for a talk at: https://talks.godoc.org/github.com/robpike/ivy/talks/ivy.slide
Video for the talk at: https://www.youtube.com/watch?v=PXoG0WX0r_E
The talk predates a lot of the features, including floating point, text, and user-defined operators.

To be built, ivy requires Go 1.17.

![Ivy](ivy.jpg)
