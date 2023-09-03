# Melt

<img width=300px  src="melt.svg">

Single file components on top of the html/template standard package.
<br>
<br>
[cheat-sheet.md](cheat-sheet.md)

## Table of Content
- [Imports](#imports)
- [Component Arguments](#component-arguments)
- [Child Components](#child-components)

## Imports
```html
<!-- component.html -->
<div>hello!</div>
```
```html
<!-- index.html -->
<import>Component component.html</import>

<h1>component</h1>
<Component />
```
- Imports can be named anything but it has to start with a Uppercase Letter.

## Component Arguments
```html
<!-- hello.html -->
<div>hello {{ .Name }}!</div>
```
```html
<!-- number.html -->
<default>$number 0</default>

<div>the number is {{ $number }}</div>
```
```html
<!-- index.html -->
<import>Hello hello.html</import>
<import>Number number.html</import>

<Hello .Name="mqix" />

<Number />
<Number $number=13 />
```
### ```$```
- Just template variables.
- If you want to set a default you can do this by declaring
it like the example above in the number.html snippet.

### ```.```
- They work as normal but if you include a component that has used them it will be required that the value is passed in by execution unless they are passed as a component argument.

### constant values
If the the argument value is a constant (numbers and strings) it will replace the variable in the template with the value.

## Child Components
```html
<!-- component.html -->
<div>
  <-Title />
  <div>
    <Slot />
  </div>
</div>
```
```html
<!-- index.html -->
<import>Component component.html</import>

<Component -Title="<h1>im a title :)</h1>">
  <div>foo<div>
  <div>bar<div>
</Component>
```
### ```<Slot />```
- All html children get placed here.
### ```<-? />```
- Starts by a - and must be followed by a Uppercase Letter
- The string from the argument with the same name gets placed here as html.
