FROM quay.io/centos/centos:stream8

RUN dnf install -y wpa_supplicant iproute-tc net-tools
RUN dnf clean all

WORKDIR /wpa_supplicant
COPY certs certs
COPY client.sh client.sh
COPY wpa_supplicant.conf wpa_supplicant.conf

RUN chmod +x /wpa_supplicant/client.sh

CMD bash
