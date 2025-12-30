# Systemd-enabled Docker image for integration testing
# See docker-systemd.md in the project root for requirements on running this container
# (requires --privileged, --cgroupns=host, tmpfs mounts, and cgroup bind mount)
FROM ubuntu:24.04

# Install systemd and required packages
RUN DEBIAN_FRONTEND=noninteractive apt-get update && \
    DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
        systemd \
        systemd-sysv \
        systemd-resolved \
        curl \
        iproute2 \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

# Configure systemd for container environment (remove unnecessary services)
RUN cd /lib/systemd/system/sysinit.target.wants/ && \
    ls | grep -v systemd-tmpfiles-setup | xargs rm -f && \
    rm -f /lib/systemd/system/multi-user.target.wants/* && \
    rm -f /etc/systemd/system/*.wants/* && \
    rm -f /lib/systemd/system/local-fs.target.wants/* && \
    rm -f /lib/systemd/system/sockets.target.wants/*udev* && \
    rm -f /lib/systemd/system/sockets.target.wants/*initctl* && \
    rm -f /lib/systemd/system/basic.target.wants/* && \
    rm -f /lib/systemd/system/anaconda.target.wants/* || true

CMD ["/lib/systemd/systemd"]
