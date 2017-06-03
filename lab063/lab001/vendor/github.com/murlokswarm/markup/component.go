package markup

import (
	"reflect"

	"github.com/murlokswarm/log"
	"github.com/pkg/errors"
	"github.com/satori/go.uuid"
)

var (
	compoBuilders = map[string]func() Componer{}
	components    = map[Componer]*component{}
	nodes         = map[uuid.UUID]*Node{}
)

// Componer is the interface that describes a component.
type Componer interface {
	// Render should returns a markup.
	// The markup can be a template string following the text/template standard
	// package rules.
	Render() string
}

// Mounter is the interface that wraps OnMount method.
// OnMount si called when a component is mounted.
type Mounter interface {
	OnMount()
}

// Dismounter is the interface that wraps OnDismount method.
// OnDismount si called when a component is dismounted.
type Dismounter interface {
	OnDismount()
}

type component struct {
	Count int
	Root  *Node
}

// Register registers a component. Allows the component to be dynamically
// created when a tag with its struct name is found into a markup.
func Register(c Componer) {
	v := reflect.ValueOf(c)

	if k := v.Kind(); k != reflect.Ptr {
		log.Panic(errors.Errorf("register accepts only components of kind %v: %v", reflect.Ptr, k))
	}

	t := v.Type().Elem()
	tag := t.Name()

	if !isComponentTag(tag) {
		log.Panic(errors.Errorf("non exported components cannot be registered: %v", t))
	}

	compoBuilders[tag] = func() Componer {
		v := reflect.New(t)
		return v.Interface().(Componer)
	}
	log.Infof("%v has been registered under the tag %v", t, tag)
}

// Registered returns true if c is registered, otherwise false.
func Registered(c Componer) bool {
	v := reflect.Indirect(reflect.ValueOf(c))
	t := v.Type()
	_, registered := compoBuilders[t.Name()]
	return registered
}

// Root returns the root node of c. Panic if c is not mounted.
func Root(c Componer) *Node {
	compo, mounted := components[c]
	if !mounted {
		log.Panic(errors.Errorf("%T is not mounted", c))
	}
	return compo.Root
}

// ID returns the id of c. Panic if c is not mounted.
func ID(c Componer) uuid.UUID {
	return Root(c).ID
}

// New creates the component named tag.
func New(tag string) (c Componer, err error) {
	b, registered := compoBuilders[tag]
	if !registered {
		err = errors.Errorf("no component named %v is registered", tag)
		return
	}
	c = b()
	return
}

// Component returns the component associated with id.
// Panic if no component with id is mounted.
func Component(id uuid.UUID) Componer {
	n, mounted := nodes[id]
	if !mounted {
		log.Panic(errors.Errorf("component with id %v is not mounted", id))
	}
	return n.Mount
}

// Markup returns the markup of c.
func Markup(c Componer) string {
	return Root(c).Markup()
}

// Mount retains a component and its underlying nodes.
func Mount(c Componer, ctx uuid.UUID) (root *Node, err error) {
	if !Registered(c) {
		err = errors.Errorf("%T is not registered", c)
		return
	}

	if compo, mounted := components[c]; mounted {
		// Go uses the same reference for different instances of a same empty struct.
		// This prevents from mounting a same empty struct.
		if t := reflect.TypeOf(c).Elem(); t.NumField() == 0 {
			compo.Count++
			root = compo.Root
			return
		}

		err = errors.Errorf("%T is already mounted", c)
		return
	}

	r, err := render(c)
	if err != nil {
		err = errors.Errorf("unable to render %T: %v\n%v", c, err, c.Render())
		return
	}

	if root, err = stringToNode(r); err != nil {
		err = errors.Errorf("%T markup returned by Render() has a %v\n%v", c, err, r)
		return
	}

	if root.Type != HTMLNode {
		err = errors.Errorf("%T markup returned by Render() has a syntax error: root node is not a HTMLNode\n%v", c, r)
		return
	}

	if err = mountNode(root, c, ctx); err != nil {
		return
	}

	components[c] = &component{
		Count: 1,
		Root:  root,
	}

	if mounter, isMounter := c.(Mounter); isMounter {
		mounter.OnMount()
	}
	return
}

func mountNode(n *Node, mount Componer, ctx uuid.UUID) error {
	switch n.Type {
	case HTMLNode:
		return mountHTMLNode(n, mount, ctx)

	case ComponentNode:
		return mountComponentNode(n, mount, ctx)
	}
	return nil
}

func mountHTMLNode(n *Node, mount Componer, ctx uuid.UUID) error {
	n.ID = uuid.NewV1()
	n.ContextID = ctx
	n.Mount = mount
	nodes[n.ID] = n

	for _, c := range n.Children {
		if err := mountNode(c, mount, ctx); err != nil {
			return err
		}
	}
	return nil
}

func mountComponentNode(n *Node, mount Componer, ctx uuid.UUID) error {
	n.ContextID = ctx
	n.Mount = mount

	c, err := New(n.Tag)
	if err != nil {
		return err
	}

	decodeAttributeMap(n.Attributes, c)

	if _, err = Mount(c, ctx); err != nil {
		return err
	}

	n.Component = c
	return nil
}

// Dismount dismounts a component.
func Dismount(c Componer) {
	compo, mounted := components[c]
	if !mounted {
		return
	}

	// Go uses the same reference for different instances of a same empty struct.
	// This prevents from dismounting an empty struct that still remains in another context.
	if compo.Count--; compo.Count == 0 {
		dismountNode(compo.Root)
		delete(components, c)

		if dismounter, isDismounter := c.(Dismounter); isDismounter {
			dismounter.OnDismount()
		}
	}
	return
}

func dismountNode(n *Node) {
	switch n.Type {
	case HTMLNode:
		dismountHTMLNode(n)

	case ComponentNode:
		Dismount(n.Component)
	}
}

func dismountHTMLNode(n *Node) {
	for _, c := range n.Children {
		dismountNode(c)
	}

	delete(nodes, n.ID)
}
