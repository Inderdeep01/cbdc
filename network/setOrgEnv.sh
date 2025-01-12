#!/bin/bash
#
# SPDX-License-Identifier: Apache-2.0




# default to using Org1
ORG=${1:-Org1}

# Exit on first error, print all commands.
set -e
set -o pipefail

# Where am I?
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )/.." && pwd )"

ORDERER_CA=${DIR}/cbdc/organizations/ordererOrganizations/cbdc/tlsca/tlsca.cbdc-cert.pem
PEER0_ORG1_CA=${DIR}/cbdc/organizations/peerOrganizations/rbi.cbdc/tlsca/tlsca.rbi.cbdc-cert.pem
PEER0_ORG2_CA=${DIR}/cbdc/organizations/peerOrganizations/hdfc.bank.cbdc/tlsca/tlsca.hdfc.bank.cbdc-cert.pem
PEER0_ORG3_CA=${DIR}/cbdc/organizations/peerOrganizations/axis.bank.cbdc/tlsca/tlsca.axis.bank.cbdc-cert.pem


if [[ ${ORG,,} == "org1" || ${ORG,,} == "rbi" ]]; then

   CORE_PEER_LOCALMSPID=RBIMSP
   CORE_PEER_MSPCONFIGPATH=${DIR}/cbdc/organizations/peerOrganizations/rbi.cbdc/users/Admin@rbi.cbdc/msp
   CORE_PEER_ADDRESS=localhost:7051
   CORE_PEER_TLS_ROOTCERT_FILE=${DIR}/cbdc/organizations/peerOrganizations/rbi.cbdc/tlsca/tlsca.rbi.cbdc-cert.pem

elif [[ ${ORG,,} == "org2" || ${ORG,,} == "hdfc" ]]; then

   CORE_PEER_LOCALMSPID=Org2MSP
   CORE_PEER_MSPCONFIGPATH=${DIR}/cbdc/organizations/peerOrganizations/hdfc.bank.cbdc/users/Admin@hdfc.bank.cbdc/msp
   CORE_PEER_ADDRESS=localhost:9051
   CORE_PEER_TLS_ROOTCERT_FILE=${DIR}/cbdc/organizations/peerOrganizations/hdfc.bank.cbdc/tlsca/tlsca.hdfc.bank.cbdc-cert.pem

elif [[ ${ORG,,} == "org3" || ${ORG,,} == "axis" ]]; then

   CORE_PEER_LOCALMSPID=Org2MSP
   CORE_PEER_MSPCONFIGPATH=${DIR}/cbdc/organizations/peerOrganizations/axis.bank.cbdc/users/Admin@axis.bank.cbdc/msp
   CORE_PEER_ADDRESS=localhost:10051
   CORE_PEER_TLS_ROOTCERT_FILE=${DIR}/cbdc/organizations/peerOrganizations/axis.bank.cbdc/tlsca/tlsca.axis.bank.cbdc-cert.pem

else
   echo "Unknown \"$ORG\", please choose Org1/rbi or Org2/hdfc or Org2/axis"
   echo "For example to get the environment variables to set upa Org2 shell environment run:  ./setOrgEnv.sh Org2"
   echo
   echo "This can be automated to set them as well with:"
   echo
   echo 'export $(./setOrgEnv.sh Org2 | xargs)'
   exit 1
fi

# output the variables that need to be set
echo "CORE_PEER_TLS_ENABLED=true"
echo "ORDERER_CA=${ORDERER_CA}"
echo "PEER0_ORG1_CA=${PEER0_ORG1_CA}"
echo "PEER0_ORG2_CA=${PEER0_ORG2_CA}"
echo "PEER0_ORG3_CA=${PEER0_ORG3_CA}"

echo "CORE_PEER_MSPCONFIGPATH=${CORE_PEER_MSPCONFIGPATH}"
echo "CORE_PEER_ADDRESS=${CORE_PEER_ADDRESS}"
echo "CORE_PEER_TLS_ROOTCERT_FILE=${CORE_PEER_TLS_ROOTCERT_FILE}"

echo "CORE_PEER_LOCALMSPID=${CORE_PEER_LOCALMSPID}"
