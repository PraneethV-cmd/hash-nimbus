# states 

-> persistent state on all servers
    -> there is currentTerm which is the latest term that the server has seen
       and this increases monotonically
    -> there is votedFor which has the candidate id that got the vote in current term
    -> there is log to maintain the log of everuthing that has happened in the node 
       and this maintained by the leader 

-> volatile state on all servers
    -> commitIndex: The index of recently commited log 
    -> lastApplied: The index of the highest log entry applied to the state machine 

-> volatile state on leaders 
    -> nextIndex[] : For each serverm, index of the next log entry to send ti leserver
    -> matchIndex[]: for each server, index of th highest log entry known to rber plicated on server and this increases monotonically 
