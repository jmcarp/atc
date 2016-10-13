port module LoginRedirect exposing (requestLoginRedirect, loginRedirect)

port requestLoginRedirect : String -> Cmd msg

port loginRedirect : (String -> msg) -> Sub msg
