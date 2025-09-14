#!/usr/bin/env bash
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
set -e
field_filter(){
     echo $1| jq '.[].displayName'| sed 's/"//g'
}
name_filter() {
    field_filter "$1" # | sed 's/Name/Alias/'
}

export donordriveRoot="https://www.extra-life.org/api/"
export fileStoragePath=${SCRIPT_DIR} # where you want the text files to be created
export topDonorPath="topdonor.txt"
export recentDonorPath="recentdonor.txt"

echo "This expects that the participant ID is stored at ${SCRIPT_DIR}/.config/participant_id"

participantId=$(cat ${SCRIPT_DIR}/.config/participant_id)
topDonorAPI="participants/${participantId}/donors?limit=1&orderBy=sumDonations%20DESC"
recentDonorAPI="participants/${participantId}/donors?limit=1&orderBy=modifiedDateUTC%20DESC"

topDonor=$(curl -s "${donordriveRoot}${topDonorAPI}")
recentDonor=$(curl -s "${donordriveRoot}${topDonorAPI}")
echo "storing at: ${fileStoragePath}/${topDonorPath}"
name_filter "${topDonor}" > "${fileStoragePath}/${topDonorPath}"
echo "storing at: ${fileStoragePath}/${recentDonorPath}"
name_filter "${recentDonor}" > "${fileStoragePath}/${recentDonorPath}"

#curl "https://www.extra-life.org/api/participants/549738/donors?limit=1&orderBy=sumDonations%20DESC" | jq'.[].displayName' > "${fileStoragePath}${topDonor}

