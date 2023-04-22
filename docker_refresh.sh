#!/bin/bash
PATH="/home/ec2-user/.vscode-server/bin/704ed70d4fd1c6bd6342c436f1ede30d1cff4710/bin/remote-cli:/usr/local/bin:/usr/bin:/usr/local/sbin:/usr/sbin:/home/ec2-user/bin:/home/ec2-user/.local/bin:/home/ec2-user/bin:/home/ec2-user/bin:/home/ec2-user/.local/bin:/home/ec2-user/bin:/home/ec2-user/bin"
cd /home/ec2-user/frothly-portal/
git pull --rebase
/bin/bash /home/ec2-user/frothly-portal/deploy.sh deploy
#git add --all 
git commit -a -m "Scheduled automatic site deployment"
git push
