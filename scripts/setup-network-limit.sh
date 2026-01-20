#!/bin/bash
# Limit bandwidth for dajtu container to 10 Mbps
# Run as root on the host after container starts
#
# Usage: sudo ./scripts/setup-network-limit.sh [container_name] [rate]
# Default: dajtu_app, 10mbit

set -e

CONTAINER_NAME="${1:-dajtu_app}"
RATE="${2:-10mbit}"
BURST="1mbit"

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    echo "Please run as root: sudo $0"
    exit 1
fi

# Get container's PID
CONTAINER_PID=$(docker inspect -f '{{.State.Pid}}' "$CONTAINER_NAME" 2>/dev/null)

if [ -z "$CONTAINER_PID" ] || [ "$CONTAINER_PID" = "0" ]; then
    echo "Error: Container '$CONTAINER_NAME' not running"
    exit 1
fi

# Find container's veth interface on host
# Method: look at the container's eth0 ifindex and find matching veth
IFINDEX=$(docker exec "$CONTAINER_NAME" cat /sys/class/net/eth0/iflink 2>/dev/null)

if [ -z "$IFINDEX" ]; then
    echo "Error: Could not determine container's network interface"
    exit 1
fi

VETH=$(ip link | grep "^${IFINDEX}:" | awk -F'[ :@]+' '{print $2}')

if [ -z "$VETH" ]; then
    # Alternative: try to find by looking at all veths
    VETH=$(ip link | grep -E "veth.*@if" | head -1 | awk -F'[ :]+' '{print $2}')
fi

if [ -z "$VETH" ]; then
    echo "Error: Could not find veth interface for container"
    exit 1
fi

# Remove existing qdisc if any
tc qdisc del dev "$VETH" root 2>/dev/null || true

# Apply traffic control - limit egress (from container's perspective: upload)
tc qdisc add dev "$VETH" root tbf rate "$RATE" burst "$BURST" latency 50ms

echo "Applied $RATE bandwidth limit to container '$CONTAINER_NAME' (interface: $VETH)"
echo "To remove: sudo tc qdisc del dev $VETH root"
