#!/bin/bash

# https://www.baeldung.com/openssl-self-signed-cert

# Create root CA
openssl req -x509 -sha256 -days 1825 -newkey rsa:2048 -keyout rootCA.key -out rootCA.crt

for n in 1 2 3; do

# Create extension
echo <<EOF > "peer$n.ext"
authorityKeyIdentifier=keyid,issuer
basicConstraints=CA:FALSE
subjectAltName = @alt_names
[alt_names]
DNS.1 = peer$n
EOF

# Create peer private key and cert signing request
openssl req -newkey rsa:2048 -nodes -keyout "peer$n.key" -out "peer$n.csr"

# Sign CSR with root cert authority, producing certificate
openssl x509 -req -CA rootCA.crt -CAkey rootCA.key -in "peer$n.csr" -out "peer$n.crt" -days 365 -CAcreateserial -extfile "peer$n.ext"
done
