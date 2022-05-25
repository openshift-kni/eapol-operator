FROM centos:stream8

RUN dnf install -y hostapd
RUN dnf clean all

COPY scripts/* /bin

ENV CONFIG=/config/hostapd.conf
CMD /usr/sbin/hostapd $CONFIG
