FROM quay.io/centos/centos:stream8

RUN dnf install -y hostapd iproute-tc
RUN dnf clean all

COPY scripts/* /bin/

ENV CONFIG=/config/hostapd.conf
CMD /usr/sbin/hostapd $CONFIG
