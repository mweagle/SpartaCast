# SpartaCast

Serverless application that creates a podcast feed from Markdown user input.
Episode content and configuration is defined in per-episode Markdown files
and rendered to speech using [AWS Polly](https://aws.amazon.com/polly/). Polly supports [SSML Tags](https://docs.aws.amazon.com/polly/latest/dg/supportedtags.html) which can be represented in a Markdown

````
```html
  <speak>
    <amazon:domain name="news">Can you believe it!</amazon:domain>

  </speak>
```
````

Podcast metadata is defined in a reserved _feed.md_ file at the root of an S3 bucket
and must include the necessary [tags](https://help.apple.com/itc/podcasts_connect/#/itcb54353390).

## Usage

```
> go get -u -v ./...
> go run main.go provision --s3Bucket $MY_S3_BUCKET
```

## Trigger

1. Create a _feed.md_ file and copy it to the user content bucket.
2. Create one or more _episodeN.md_ files and copy them adjacent to the _feed.md_ file.
3. Monitor results in the [Step Functions Console](https://aws.amazon.com/step-functions/)
4. Add the public S3 URL to the _/public/feed/feed.xml_ to your [favorite podcast player](https://medium.com/@joshmuccio/how-to-manually-add-a-rss-feed-to-your-podcast-app-on-desktop-ios-android-478d197a3770).

## Details

The application provisions two S3 buckets, one for user assets and the other for
CloudTrail logs.

The CloudTrail bucket is used to create a Trail that can be used to directly [trigger
an StepFunction from an S3 event](https://docs.aws.amazon.com/step-functions/latest/dg/tutorial-cloudwatch-events-s3.html). This bucket includes _cloudtrailstorage_ in the name.


The user asset bucket includes _eventbucket_ in the name and is used to store user content.
The bucket keyspace is partitioned into the following scopes.

* / (root) **PRIVATE**
  * Source user content
  * Examples: _feed.md_, _episode1.md_
* /public **PUBLIC**
  * /feed
    * Generated podcast
    * Polly generated MP3s
  * /metadata
    * Intermediate metadata files

When a new _episode.md_ is uploaded to the event bucket, it triggers a CloudTrail event, which is subscribed to by the [EventPattern](https://github.com/mweagle/SpartaCast/blob/master/infra/eventpattern_put.json) rule that then invokes, via EventBridge, the rendering and feed generation Step function:

<div align="center"><img src="https://raw.githubusercontent.com/mweagle/SpartaCast/master/site/describe.jpeg" />
</div>

<div align="center"><img src="https://raw.githubusercontent.com/mweagle/SpartaCast/master/site/step.png" />
</div>

See the [lambda.go](https://github.com/mweagle/SpartaCast/blob/master/lambda/lambda.go) source file for the full set of recognized properties.

# Markson Configuration

The Markdown configuration represents a flat Key-Value space. Key-Value pairs can be represented in two different ways:

* An `H1` keyname, whose entire content represents the value.
* A reserved `H1` header named _Properties_ that can represent multiple Key-Value pairs in a Markdown table. See the sample files in [/media](https://github.com/mweagle/SpartaCast/tree/master/media).
