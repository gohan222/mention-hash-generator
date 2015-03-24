package main

/********************************************
*
*	Database Model
*
*******************************************/

type DbMention struct {
	MentionId *int64  `db:"mention_id"`
	Snippets  *string `db:"mention_snippets"`
}
