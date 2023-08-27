# Melt
  svelte like component files for html/template standaard package

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
- Imports can be named anything but it has to start with a Uppercase Letter

## Component Arguments
```html
<!-- hello.html -->
<div>hello {{ .Name }!</div>
```
```html
<!-- number.html -->
{{$number = 0 }}
<div>the number is {{ $number }}</div>
```

```html
<!-- index.html -->
<import>Hello hello.html</import>
<import>Number number.html</import>

<!-- <Component />  this does not work -->
<Component .Name="mqix" />

<Number /> <!-- this works -->
<Number $number=13 />
```
