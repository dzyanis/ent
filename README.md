# ent

Ent provides a small HTTP interface to manage blobs namespace partitioned by buckets. Depending on the FileSystem implementation used it needs to run as a single instance per host or as many instances scaled out horizontally.

Within a bucket, you can use any names for your objects, but bucket names must be unique. Only one Owner can exist per Bucket.

## API

**POST** `/{bucket}/{key}` - Provide a request body with the binary data of the blob you want to store.

```
$ curl -s -X POST --data-binary @mybig.blob \
    'http://localhost:5555/ent/my/big.blob
{
  "duration": 12000000,
  "file": {
    "bucket": {
      "name": "ent",
      "owner": {...}
    },
    "key":    "my/big.blob",
    "sha1":   "e9f6f0657f6d33aa15cfd885bc34713a266a729a"
  }
}
```

**GET** `/{bucket}/{key}` - Returns the blob data in binary format in the response body.

```
$ curl -s 'http://localhost:5555/ent/my/big.blob > big.blob
$ sha1sum big.blob
e9f6f0657f6d33aa15cfd885bc34713a266a729a  big.blob
```

**GET** / - Returns the list of existing buckets.

```
$ curl -s 'http://localhost:5555/
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
```

**GET** `/{bucket}?prefix={prefix}&sort=+key&limit={limit}` - Lists the blobs in a bucket.

***Parameter's description***

 1) *prefix* 
- Lists only the blobs with the given prefix. Type: String. Default: ""

 2) *sort*
- #{"+lastModified", "-lastModified", "+key", "-key"} Specifies the sorting criteria. When set to lastModified, the  blobs are sorted by latest modified. If no value is defined, the order of the blobs is not guaranteed. Type: string. Default: "".
- starting with +/-, the list will be sorted in ascending/descending order.

 3) *limit*
- maximum number of the files returned. Default: All the files are returned.

```
$ curl -s 'http://localhost:5555/ent?prefix=prefix1%2Fprefix2&sort=%2BlastModified&limit=2
$ 
{
    "bucket": {
        "name": "bit",
        "owner": {
            "email": {
                "Address": "bit@bucket.io",
                "Name": "bit team"
            }
        }
    },
    "count": 2,
    "duration": 367649,
    "files": [
        {
            "bucket": {
                "name": "bit",
                "owner": {
                    "email": {
                        "Address": "bit@bucket.io",
                        "Name": "bit team"
                    }
                }
            },
            "key": "prefix1/prefix2/big.blob",
            "lastModified": "2014-08-28T16:29:06+02:00",
            "sha1": "0def144a75d76e89bb91fc7797d140f1d103ffb9"
        },
        {
            "bucket": {
                "name": "bit",
                "owner": {
                    "email": {
                        "Address": "bit@bucket.io",
                        "Name": "bit team"
                    }
                }
            },
            "key": "prefix1/prefix2big.blob",
            "lastModified": "2014-08-28T16:16:40+02:00",
            "sha1": "c85320d9ddb90c13f4a215f1f0a87b531ab33310"
        }
    ]
}
```

## DESIGN

Ent is organised around the FileSystem interface which supports a CRUD feature set. This should give enough flexibility to use implementations ranging from disk based to S3, even a Content-addressable storage could be imagined. To ensure stability for the FileSystem interface we only assume Bucket and Key. Where it is up to the actual FS implementation how it handles namespace partitioning based on the Bucket information.

The Bucket requires an Owner and always only has one. It is this type where future concepts should be incorporated like quota handling, permissions, etc.
