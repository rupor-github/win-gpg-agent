package server

import (
	"log"

	"win-gpg-agent/assuan/common"
)

// Inquire requests data with specified keywords from client.
//
// It's better to explain by example:
//  Inquire(pipe, []byte{"KEYBLOCK", "KEYBLOCK_INFO"})
//
//  Will result in following messages sent by peers (S - server, C - client):
//  S: INQUIRE KEYBLOCK
//  C: D ...
//  C: END
//  S: INQUIRE KEYBLOCK_INFO
//  C: D ...
//  C: END
//
// Note: No OK or ERR sent after completion. You must report errors returned
// by this function manually using WriteError or send OK.
// This function can return common.Error, so you can do the following:
//	 data, err := server.Inquire(scnr, pipe, ...)
//	 if err != nil {
//	     if e, ok := err.(common.Error); ok {
//	  	   // Protocol error, report it to other peer (client).
//	       pipe.WriteError(e)
//	     } else {
//	  	   // Internal error, do something else...
//	     }
//	 }
func Inquire(pipe *common.Pipe, keywords []string) (res map[string][]byte, err error) {
	res = make(map[string][]byte)

	log.Println("Sending inquire group:", keywords)
	for _, keyword := range keywords {
		if err := pipe.WriteLine("INQUIRE", keyword); err != nil {
			log.Println("... I/O error:", err)
			return nil, err
		}

		data, err := pipe.ReadData()
		if err != nil {
			perr, ok := err.(*common.Error)

			if ok {
				if err := pipe.WriteError(*perr); err != nil {
					log.Println("... I/O error:", err)
					return nil, err
				}
				return nil, err
			}

			log.Println("... I/O error:", err)
			return nil, err
		}

		res[keyword] = data
	}
	return res, nil
}
