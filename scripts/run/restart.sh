#!/bin/bash
# Made by YTSworks
# YTS工作室製作
pkill -f Management Server
sleep 2
sqlite3 /home/ubuntu/Management Server/data/nms.db < /tmp/reset_key.sql
cd /home/ubuntu/Management Server/backend
go build -o Management Server .
nohup ./Management Server > /tmp/nms.log 2>&1 &
echo "Service restarted."
