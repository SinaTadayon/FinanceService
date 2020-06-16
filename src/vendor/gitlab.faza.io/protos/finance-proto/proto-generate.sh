#!/bin/bash

# Author: Mostafa Talebi
# Desc: Generates proto files and tags for the specified file
DIR=./
FILE=message
if [ "$#" == 0 ]
then
    echo "No flag is provided. Pass name of a file!"
    echo "Help"
    echo "A file name without the proto extension must be passed, the base directory is $DIR"
    echo "Use --skip-tag if you do not want have any tags in your proto file."
    exit 1;
fi;

if [[ ! -f "$DIR$FILE.proto" ]]
then
    echo "The file does not exists."
    exit 1;
fi;

`protoc -I=. --go_out=plugins=grpc:. $FILE.proto`
`mv $FILE.pb.go $FILE.go`
echo "Generated the proto file. $DIR for the generated file."
if [[ $2 != "--skip-tag" ]]; then
    `protoc-go-inject-tag -input=$DIR$FILE.go`
    echo "Generated tags for the $DIR$FILE.pb.go"
fi;

exit 0
