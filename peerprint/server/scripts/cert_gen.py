# pip install pyOpenSSL
from OpenSSL import crypto, SSL
from pathlib import Path

def gen_ca_cert(emailAddress="noreply@peerprint", commonName="peerprint-server", countryName="NA", localityName="NA", stateOrProvinceName="NA", organizationName="PeerPrint", organizationUnitName="PeerPrint", validityStartInSeconds=0, validityEndInSeconds=10*365*24*60*60):
    #can look at generated file using openssl:
    #openssl x509 -inform pem -in selfsigned.crt -noout -text
    k = crypto.PKey()
    k.generate_key(crypto.TYPE_RSA, 4096)
    cert = crypto.X509()
    cert.add_extensions([
        crypto.X509Extension("basicConstraints".encode("utf8"), False, "CA:TRUE".encode("utf8")),
        # crypto.X509Extension("keyUsage", False, "keyCertSign, cRLSign"),
    ])

    cert.get_subject().C = countryName
    cert.get_subject().ST = stateOrProvinceName
    cert.get_subject().L = localityName
    cert.get_subject().O = organizationName
    cert.get_subject().OU = organizationUnitName
    cert.get_subject().CN = commonName
    cert.get_subject().emailAddress = emailAddress
    cert.set_serial_number(0)
    cert.gmtime_adj_notBefore(0)
    cert.gmtime_adj_notAfter(validityEndInSeconds)
    cert.set_issuer(cert.get_subject())
    cert.set_pubkey(k)
    cert.sign(k, 'sha512')
    return  cert, k

def gen_cert(ca_cert, ca_key, sernum):
    k = crypto.PKey()
    k.generate_key(crypto.TYPE_RSA, 2048)

    req = crypto.X509Req()
    req.get_subject().C = ca_cert.get_subject().C
    req.get_subject().ST = ca_cert.get_subject().ST
    req.get_subject().L = ca_cert.get_subject().L
    req.get_subject().O = ca_cert.get_subject().O
    req.get_subject().OU = ca_cert.get_subject().OU
    req.get_subject().CN = "peerprint"  # https://stackoverflow.com/a/19738223
    req.get_subject().emailAddress = ca_cert.get_subject().emailAddress
    req.set_pubkey(k)
    req.sign(k, 'sha512')

    certs = crypto.X509()
    certs.set_serial_number(sernum)
    certs.gmtime_adj_notBefore(0)
    certs.gmtime_adj_notAfter(10*365*24*60*60)
    certs.set_subject(req.get_subject())
    certs.set_issuer(ca_cert.get_subject())
    certs.set_pubkey(k)
    certs.sign(ca_key, 'sha512')
    return certs, k

def dump(cert, k, cert_file, key_file):
    with open(cert_file, "wt") as f:
        f.write(crypto.dump_certificate(crypto.FILETYPE_PEM, cert).decode("utf-8"))
    with open(key_file, "wt") as f:
        f.write(crypto.dump_privatekey(crypto.FILETYPE_PEM, k).decode("utf-8"))

def gen_certs(outdir, ncli):
    outdir = Path(outdir)
    ca_cert, ca_k = gen_ca_cert()
    dump(ca_cert, ca_k, outdir/"ca_cert.pem", outdir/"ca_key.pem")
    
    server_cert, server_k = gen_cert(ca_cert, ca_k, 1)
    dump(server_cert, server_k, outdir/f"server_cert.pem", outdir/f"server_key.pem")


    for i in range(ncli):
        cert, k = gen_cert(ca_cert, ca_k, 2+i)
        dump(cert, k, outdir/f"client{i}_cert.pem", outdir/f"client{i}_key.pem")

if __name__ == "__main__":
    import sys
    ca_cert, ca_k = gen_ca_cert()
    dump(ca_cert, ca_k, "server_cert.pem", "server_key.pem")
    ncli = 1
    if len(sys.argv) == 2:
        ncli = int(sys.argv[1])
    gen_certs(".", ncli)

    
