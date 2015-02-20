dbox
====

Command Line Interface for Dropbox.

Prerequisite
------------

To use this program, you must have a valid client ID (app key) and client secret (app secret) provided by Dropbox.
To register a new client application, please visit https://www.dropbox.com/developers/apps/create
Set the corresponding variables in the source (APP_KEY and APP_SECRET).

Configuration File
------------------

This program uses a configuration file located at ~/.dbox.
The format is the following:

	{
	 "token": "OAuth2 authentication token",
	 "key": "key_generated_for_encrypting_and_decrypting_files"
	}

Usage
-----

	$ go run dbox.go help
	Command list:
	      copy: Copy file or directory.
		    Usage: copy [-r] from_file to_file
	   copyref: Get a copy reference of a file.
		    Usage: copyref file [files...]
	      cput: Upload a file.
		    Usage: cput [-aes] [-c chunksize] [-k] [-r rev] [-t trycount] file destination
	    delete: Remove file or directory (Warning this remove is recursive).
		    Usage: delete file [files...]
	     delta: Get modifications.
		    Usage: delta [-c cursor] [-p path_prefix]
	       get: Download a file.
		    Usage: get [-aes] [-c] [-r rev] file destination
	    ldelta: Get modifications with timeout.
		    Usage: ldelta [-t timeout] cursor
	      list: List files from directories.
		    Usage: list [-a] [-d] [-l] file [files...]
	     media: Shares files with direct access.
		    Usage: media file [files...]
	     mkdir: Create directories.
		    Usage: mkdir directory [directories...]
	      move: Move file or directory.
		    Usage: move from_file to_file
	       put: Upload a file.
		    Usage: put [-aes] [-k] [-r rev] file destination
	   restore: Restore a file to a previous revision.
		    Usage: restore path revision
	 revisions: Get revisions of files.
		    Usage: revisions [-l] file destination
	    search: Search files.
		    Usage: search [-a] [-l] [-m limit] path "query words"
	    shares: Share files.
		    Usage: shares [-o] file [files...]
	thumbnails: Download a thumbnail.
		    Usage: thumbnails [-s size] [-f format] files destination
	      help: Show this help message
