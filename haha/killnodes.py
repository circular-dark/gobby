#!/bin/python
import subprocess

netstat = "netstat -pnl"
kill = "kill "
s = subprocess.check_output(netstat.split())
for line in s.split('\n'):
  if ":::54328" in line or ":::54327" in line or ":::54326" in line or ":::54325" in line or ":::54324" in line or ":::54323" in line or ":::54322" in line:
    pid = line.split()[-1].split('/')[0]
    kill += pid + " "
print kill
subprocess.call(kill.split())
