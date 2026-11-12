# Keychain spike

This folder contains throwaway code for issue #53.
It is not production code. Do not use it in the product.

`kcspike` calls Security.framework through `purego`, without CGO.
It only handles dummy items whose service name starts with `a38-spike-`.
It reads a value from stdin and prints only the length of a value that it reads.

The report is in `docs/explanation/spikes/50-token-im-schluesselbund.md`.

## Build

```sh
CGO_ENABLED=0 go build -o bin/kcspike-a .
CGO_ENABLED=0 go build -ldflags "-X main.variant=b" -o bin/kcspike-b .
```

## Commands

```sh
printf %s spike-dummy-wert-1 | bin/kcspike-a add -s a38-spike-x
bin/kcspike-a read   -s a38-spike-x
bin/kcspike-a acl    -s a38-spike-x
bin/kcspike-a delete -s a38-spike-x
printf %s spike-dummy-wert-1 | bin/kcspike-a add -dp -s a38-spike-y
```

`read` from a second build (`kcspike-b`) shows a keychain dialog.
`-noui` does not prevent this dialog for an item in the file-based keychain.
