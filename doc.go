// Copyright (c) 2014, SoundCloud Ltd.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.
// Source code and contact info at http://github.com/soundcloud/ent

/*
Ent provides a small HTTP interface to manage blobs namespace partitioned by
buckets. Depending on the FileSystem implementation used ent needs to run as a
single instance per host or as many instnaces scaled out horizontally.

Within a bucket, you can use any names for your objects, but bucket names must
be unique. Only one Owner can exist per Bucket.

API

POST /{bucket}/{key} - Provide a request body with the binary data of the blob
you want to store.

  $ curl -s -X POST --data-binary @mybig.blob \
      'http://localhost:5555/ent/my/big.blob
  {
    "duration": "12000000",
    "file": {
      "bucket": {
        "name": "ent",
        "owner": {...}
      },
      "key":    "my/big.blob",
      "sha1":   "e9f6f0657f6d33aa15cfd885bc34713a266a729a"
    }
  }

GET /{bucket}/{key} - Returns the blob data in binary format in the response
body.

  $ curl -s 'http://localhost:5555/ent/my/big.blob > big.blob
  $ sha1sum big.blob
  e9f6f0657f6d33aa15cfd885bc34713a266a729a  big.blob

GET / - Returns the list of existing buckets.

  $ curl -s 'http://localhost:555/
  {
    "count": 3,
    "duration": 2404,
    "buckets": [
      {
        "owner": {
          "email": {
            "Address": "bit@ent.io",
            "Name": "bit team"
          }
        },
        "name": "bit"
      },
      ...
    ]
  }

DESIGN

Ent is organised around the FileSystem interface which supports a CRUD feature
set. This should give enough flexibility to use implementations ranging from
disk based to S3, even a Content-addressable storage could be imagined. To
ensure stability for the FileSystem interface we only assume Bucket and Key.
Where it is up to the actual FS implementation how it handles namespace
partitioning based on the Bucket information.

The Bucket requires an Owner and always only has one. It is this type where
future concepts should be incorporated like quota handling, permissions, etc.
*/
package main
