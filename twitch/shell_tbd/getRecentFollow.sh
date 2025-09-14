#!/usr/bin/env bash
### This utility was written by FredS104 @spospartan104 on twitter and github
### It uses the Twitch shell client to grab the recent follower from the specified account
### It expects your (twitch cli)[https://github.com/twitchdev/twitch-cli] to already be configured. 
### (these will be checked in bootstrap.sh)

# Check for number of args
if [[ "$#" -eq "1" ]]; then 
    last_known_follow=$1
else
    last_known_follow=""
fi

json_resp=$(twitch api get users follows -q to_id=${Twitch_helper_chan_id} -q first=1)
# Relevant fields
#       "followed_at": "UTCZ Date stamp",
#       "from_id": "user_id",
#       "from_login": "user_name",
#       "from_name": "user_name" 

follower_name=$(echo ${json_resp}|jq '.data[0].from_name')
if [[ "${follower_name}" == "\"${last_known_follow}\"" ]]; then
  exit 0
else
  echo "${follower_name}"
  exit 0
fi