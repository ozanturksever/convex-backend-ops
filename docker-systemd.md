# Docker Images with Systemd

This guide explains how to create Docker images that run systemd as PID 1 and how to properly run containers using those images.

## Overview

Running systemd inside Docker containers allows you to:
- Manage services using `systemctl`
- Run multiple services with proper dependency ordering
- Use systemd timers, socket activation, and other features
- Test systemd-based applications in isolated environments

## Building a Systemd-Enabled Docker Image

### Minimal Dockerfile

```dockerfile
FROM ubuntu:24.04

# Install systemd
RUN DEBIAN_FRONTEND=noninteractive apt-get update && \
    DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
        systemd \
        systemd-sysv \
        systemd-resolved \
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
    rm -f /lib/systemd/system/anaconda.target.wants/*

# Set systemd as the entrypoint
CMD ["/lib/systemd/systemd"]
```

### Key Dockerfile Components

#### 1. Install Systemd Packages

```dockerfile
RUN DEBIAN_FRONTEND=noninteractive apt-get update && \
    DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
        systemd \
        systemd-sysv \
        systemd-resolved \
        iproute2 \
        iputils-ping \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*
```

- `systemd` - The init system
- `systemd-sysv` - SysV init compatibility
- `systemd-resolved` - DNS resolution service (optional but recommended)

#### 2. Remove Unnecessary Services

Containers don't need hardware-related services. Remove them to speed up boot and avoid errors:

```dockerfile
RUN cd /lib/systemd/system/sysinit.target.wants/ && \
    ls | grep -v systemd-tmpfiles-setup | xargs rm -f && \
    rm -f /lib/systemd/system/multi-user.target.wants/* && \
    rm -f /etc/systemd/system/*.wants/* && \
    rm -f /lib/systemd/system/local-fs.target.wants/* && \
    rm -f /lib/systemd/system/sockets.target.wants/*udev* && \
    rm -f /lib/systemd/system/sockets.target.wants/*initctl* && \
    rm -f /lib/systemd/system/basic.target.wants/* && \
    rm -f /lib/systemd/system/anaconda.target.wants/*
```

#### 3. Mask Problematic Services

Some services cannot function in containers. Mask them to prevent startup failures:

```dockerfile
# Mask auditd (requires kernel audit support not available in containers)
RUN systemctl mask auditd.service 2>/dev/null || true
```

#### 4. Enable User Sessions (Optional)

If you need SSH or interactive logins:

```dockerfile
RUN mkdir -p /etc/systemd/system/multi-user.target.wants && \
    ln -sf /usr/lib/systemd/system/systemd-user-sessions.service \
    /etc/systemd/system/multi-user.target.wants/systemd-user-sessions.service
```

#### 5. Set Systemd as CMD

```dockerfile
CMD ["/lib/systemd/systemd"]
```

### Complete Example with SSH

```dockerfile
FROM ubuntu:24.04

# Install systemd and SSH
RUN DEBIAN_FRONTEND=noninteractive apt-get update && \
    DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
        systemd \
        systemd-sysv \
        systemd-resolved \
        openssh-server \
        iproute2 \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

# Configure systemd for container
RUN cd /lib/systemd/system/sysinit.target.wants/ && \
    ls | grep -v systemd-tmpfiles-setup | xargs rm -f && \
    rm -f /lib/systemd/system/multi-user.target.wants/* && \
    rm -f /etc/systemd/system/*.wants/* && \
    rm -f /lib/systemd/system/local-fs.target.wants/* && \
    rm -f /lib/systemd/system/sockets.target.wants/*udev* && \
    rm -f /lib/systemd/system/sockets.target.wants/*initctl* && \
    rm -f /lib/systemd/system/basic.target.wants/* && \
    rm -f /lib/systemd/system/anaconda.target.wants/*

# Configure SSH
RUN sed -i 's/#PermitRootLogin prohibit-password/PermitRootLogin yes/' /etc/ssh/sshd_config && \
    sed -i 's/#PasswordAuthentication yes/PasswordAuthentication yes/' /etc/ssh/sshd_config && \
    mkdir -p /var/run/sshd && \
    ssh-keygen -A && \
    systemctl enable ssh

# Enable user sessions for SSH login
RUN mkdir -p /etc/systemd/system/multi-user.target.wants && \
    ln -sf /usr/lib/systemd/system/systemd-user-sessions.service \
    /etc/systemd/system/multi-user.target.wants/systemd-user-sessions.service

# Set root password (change in production!)
RUN echo 'root:password' | chpasswd

CMD ["/lib/systemd/systemd"]
```

## Running Systemd Containers

### Required Docker Run Flags

Systemd requires specific privileges and configurations to run properly:

```bash
docker run -d \
    --privileged \
    --cgroupns=host \
    --tmpfs /run \
    --tmpfs /run/lock \
    -v /sys/fs/cgroup:/sys/fs/cgroup:rw \
    --name my-systemd-container \
    my-systemd-image
```

### Flag Explanations

| Flag | Purpose |
|------|---------|
| `--privileged` | Grants extended privileges needed for systemd |
| `--cgroupns=host` | Uses the host's cgroup namespace (required for systemd cgroup management) |
| `--tmpfs /run` | Mounts tmpfs at /run (systemd runtime directory) |
| `--tmpfs /run/lock` | Mounts tmpfs at /run/lock (lock files) |
| `-v /sys/fs/cgroup:/sys/fs/cgroup:rw` | Mounts cgroup filesystem read-write |

### Docker Compose Example

```yaml
version: '3.8'

services:
  systemd-container:
    image: my-systemd-image
    privileged: true
    cgroup_parent: docker.slice
    tmpfs:
      - /run:rw,noexec,nosuid
      - /run/lock:rw,noexec,nosuid
    volumes:
      - /sys/fs/cgroup:/sys/fs/cgroup:rw
    stdin_open: true
    tty: true
```

### Programmatic Example (Go with Docker SDK)

```go
import (
    "github.com/docker/docker/api/types/container"
)

config := &container.Config{
    Image: "my-systemd-image",
    Cmd:   []string{"/lib/systemd/systemd"},
    Tty:   true,
}

hostConfig := &container.HostConfig{
    Privileged:   true,
    CgroupnsMode: "host",
    Tmpfs: map[string]string{
        "/run":      "rw,noexec,nosuid",
        "/run/lock": "rw,noexec,nosuid",
    },
    Binds: []string{
        "/sys/fs/cgroup:/sys/fs/cgroup:rw",
    },
}

resp, err := client.ContainerCreate(ctx, config, hostConfig, nil, nil, "my-container")
```

## Working with Systemd Containers

### Executing Commands

```bash
# Run a command
docker exec my-systemd-container systemctl status

# Interactive shell
docker exec -it my-systemd-container bash
```

### Managing Services

```bash
# Check service status
docker exec my-systemd-container systemctl status nginx

# Start a service
docker exec my-systemd-container systemctl start nginx

# Enable service at boot
docker exec my-systemd-container systemctl enable nginx

# View logs
docker exec my-systemd-container journalctl -u nginx -f
```

### Checking Systemd Status

```bash
# List all services
docker exec my-systemd-container systemctl list-units --type=service

# Check boot status
docker exec my-systemd-container systemctl is-system-running

# View boot log
docker exec my-systemd-container journalctl -b
```

## Best Practices

### 1. Wait for Systemd Initialization

After starting a container, wait for systemd to fully initialize:

```bash
# Wait for systemd to be ready
sleep 5

# Or check programmatically
docker exec my-container systemctl is-system-running --wait
```

### 2. Use Service Drop-in Files

Configure services using drop-in files instead of modifying main unit files:

```dockerfile
# Create drop-in directory
RUN mkdir -p /etc/systemd/system/myservice.service.d

# Add configuration override
RUN echo '[Service]\nEnvironment="MY_VAR=value"' > \
    /etc/systemd/system/myservice.service.d/override.conf
```

### 3. Handle Package Installation Timing

When installing packages that include systemd services, the order matters:

```dockerfile
# Install packages FIRST
RUN apt-get update && apt-get install -y mypackage

# Configure services AFTER installation
# (packages may recreate service directories)
RUN mkdir -p /etc/systemd/system/multi-user.target.wants && \
    ln -sf /usr/lib/systemd/system/myservice.service \
    /etc/systemd/system/multi-user.target.wants/myservice.service
```

### 4. Resource Limits

Set appropriate resource limits for containerized services:

```dockerfile
RUN mkdir -p /etc/systemd/system/elasticsearch.service.d && \
    echo '[Service]\nEnvironment="ES_JAVA_OPTS=-Xms1g -Xmx1g"' > \
    /etc/systemd/system/elasticsearch.service.d/memory.conf
```

### 5. Graceful Shutdown

For clean shutdown, stop the container with a timeout:

```bash
docker stop --time=30 my-systemd-container
```

## Troubleshooting

### Container Won't Start

1. **Check cgroup version**: Modern systemd requires cgroup v2 or unified hierarchy
   ```bash
   # Check cgroup version on host
   mount | grep cgroup
   ```

2. **Verify privileged mode**: Systemd needs `--privileged` flag

3. **Check tmpfs mounts**: Ensure `/run` and `/run/lock` are mounted as tmpfs

### Services Fail to Start

1. **Check service logs**:
   ```bash
   docker exec my-container journalctl -u myservice -n 50
   ```

2. **Check for masked services**:
   ```bash
   docker exec my-container systemctl list-unit-files | grep masked
   ```

3. **Verify dependencies**:
   ```bash
   docker exec my-container systemctl list-dependencies myservice
   ```

### Slow Boot

1. **Remove unnecessary services** (see Dockerfile configuration above)

2. **Check what's taking time**:
   ```bash
   docker exec my-container systemd-analyze blame
   ```

### "Failed to connect to bus" Errors

This usually means systemd isn't running or hasn't initialized:
- Ensure `CMD ["/lib/systemd/systemd"]` is set
- Wait for systemd to initialize before running commands
- Check that the container started successfully

## Security Considerations

Running systemd containers requires `--privileged` mode, which has security implications:

1. **Container Isolation**: Privileged containers have more access to the host
2. **Network Segmentation**: Use Docker networks to isolate containers
3. **Resource Limits**: Set CPU and memory limits
4. **Image Trust**: Only use trusted base images

For production environments, consider:
- Using dedicated VMs for systemd workloads
- Implementing additional security controls
- Regular security audits of container configurations

## References

- [systemd Documentation](https://www.freedesktop.org/wiki/Software/systemd/)
- [Docker Documentation](https://docs.docker.com/)
- [Running systemd in a container](https://systemd.io/CONTAINER_INTERFACE/)
