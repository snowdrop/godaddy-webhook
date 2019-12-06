#!/bin/bash

GITHUB_API_TOKEN=$1
CURRENT_TAG=$2

OWNER="snowdrop"
REPO="webhook-godaddy"
AUTH="Authorization: token $GITHUB_API_TOKEN"
GH_API="https://api.github.com"
GH_REPO="$GH_API/repos/$OWNER/$REPO"
GH_TAGS="$GH_REPO/releases/tags/$CURRENT_TAG"

# perform github api lookup for tag
response=$(curl -sH "$AUTH" $GH_TAGS)

# Get ID of the asset based on given filename.
eval $(echo "$response" | grep -m 1 "id.:" | grep -w id | tr : = | tr -cd '[[:alnum:]]=')
[ "$id" ] || { echo "Error: Failed to get release id for tag: $CURRENT_TAG"; echo "$response" | awk 'length($0)<100' >&2; exit 1; }

# ensure that we have same tags locally as are on remote
git fetch --tags
git fetch --prune origin "+refs/tags/*:refs/tags/*"

PREVIOUS_TAG=$(git describe --abbrev=0 ${CURRENT_TAG}^ --tags)

# we use the contents of the commits between the current and previous tag as the release body
RELEASE_BODY=$(git log --decorate=short --pretty=format:'%h %d%  %s\n' ${PREVIOUS_TAG}..${CURRENT_TAG} | sed -e "s/(.*tag: ${CURRENT_TAG}.*) /(tag: ${CURRENT_TAG}) /g" | paste -sd "" -)

GH_RELEASE_URL="${GH_REPO}/releases/$id"
echo "GH_RELEASE_URL : $GH_RELEASE_URL"

curl -i -H "Authorization: token $GITHUB_API_TOKEN" \
     -H "Content-Type: application/json" \
     -X PATCH \
     --data "{\"body\": \"${RELEASE_BODY}\"}" \
     $GH_RELEASE_URL

