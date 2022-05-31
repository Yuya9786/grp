#!/bin/sh

vtysh -c "conf t" \
          -c "router bgp 100" \
          -c "bgp router-id 1.1.1.1" \
          -c "neighbor 10.0.0.3 remote-as 200"
