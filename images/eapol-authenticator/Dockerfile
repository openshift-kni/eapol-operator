FROM centos:stream8

RUN dnf install -y hostapd
RUN dnf clean all

ENV CONFIG=/config/hostapd.conf
CMD /usr/sbin/hostapd $CONFIG
