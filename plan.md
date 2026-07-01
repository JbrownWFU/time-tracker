**Overview**
Simple go time tracking CLI. Create, Close, and track time spent on tasks / projects / jobs.

**Packages**
[modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite?utm_source=godoc)
[Kong CLI](https://github.com/alecthomas/kong)

**Structures**
### Job
A job is a task / project
#### Properties:
>   - ID: Unique identifier
>   - Name: Job name
>   - Description: Optional long description
>   - Status: One of `TODO`, `ACTIVE`, `DONE`

### Entry
An entry is a time spent on a job

#### Properties:
>   - ID: Unique identifer
>   - Job ID: Foreign key to Job table
>   - StartTime: Start timestamp
>   - Endtime: End timestamp - null until clocked out
>   - Notes: Optional notes entry 

**Architecture**
### General Usage
`track [command] <arguments>`

### Job Management
Create a new job with name
`track create <name>`

Update a job status
`track status <name> {enum: todo, active, done}`

Print the full details of a job
`track show <name>`

### Time Management
Start time on a job
`track in <job>`

Stop time on a job and optionally store notes
`track out <job> <notes>`