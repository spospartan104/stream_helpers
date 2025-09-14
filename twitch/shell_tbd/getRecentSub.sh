#!/usr/bin/env bash
### This utility was written by FredS104 @spospartan104 on twitter and github
### It uses the Twitch shell client to grab the recent subscriber from the specified account
### It expects your (twitch cli)[https://github.com/twitchdev/twitch-cli] to already be configured. 
### (these will be checked in bootstrap.sh)


### NOTE This can only be run on channels you have oauth scope for. You'll need a user token
### From twitch cli this can be done with: twitch token -u -s "channel:read:subscriptions"

#TODO: add in logic to check if auth'd (error in 400s)

# Check for number of args
if [[ "$#" -eq "1" ]]; then 
    last_known_sub=$1
else
    last_known_sub=""
fi

json_resp=$(twitch api get users subscribers -q to_id=${Twitch_helper_chan_id} -q first=1)
# Relevant fields
#       "followed_at": "UTCZ Date stamp",
#       "from_id": "user_id",
#       "from_login": "user_name",
#       "from_name": "user_name" 

follower_name=$(echo ${json_resp}|jq '.data[0].from_name')
if [[ "${subscriber_name}" == "\"${last_known_sub}\"" ]]; then
  exit 0
else
  echo "${subscriber_name}"
  exit 0
fi