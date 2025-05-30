#!/usr/bin/env bash

set -o errexit
set -o nounset
# set -o xtrace

# Positional arguments:
#
# 1. blinkdisk_recovery_dir
# 2. blinkdisk_exe_dir
# 3. test_duration
# 4. test_timeout
# 5. test_repo_path_prefix

# Environment variables that modify the behavior of the recovery job execution
#
# - AWS_ACCESS_KEY_ID: To access the repo bucket
# - AWS_SECRET_ACCESS_KEY: To access the repo bucket
# - FIO_EXE: Path to the fio executable, if unset a Docker container will be
#       used to run fio.
# - HOST_FIO_DATA_PATH:
# - LOCAL_FIO_DATA_PATH: Path to the local directory where snapshots should be
#       restored to and fio data should be written to
# - S3_BUCKET_NAME: Name of the S3 bucket for the repo

readonly blinkdisk_recovery_dir="${1?Specify directory with blinkdisk recovery git repo}"
readonly blinkdisk_exe_dir="${2?Specify the directory of the blinkdisk git repo to be tested}"

readonly test_duration=${3:?"Provide a minimum duration for the testing, e.g., '15m'"}
readonly test_timeout=${4:?"Provide a timeout for the test run, e.g., '55m'"}
readonly test_repo_path_prefix=${5:?"Provide the path that contains the data and metadata repos"}

# Remaining arguments are additional optional test flags
shift 5

cat <<EOF
--- Job parameters ----
blinkdisk_recovery_dir: '${blinkdisk_recovery_dir}'
blinkdisk_exe_dir: '${blinkdisk_exe_dir}'
test_duration: '${test_duration}'
test_timeout: '${test_timeout}'
test_repo_path_prefix: '${test_repo_path_prefix}'
additional_args: '${@}'

--- Optional Job Parameters via Environment Variables ---
AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID-}
AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY:+<xxxx>}
ENGINE_MODE=${ENGINE_MODE-}
FIO_EXE=${FIO_EXE-}
HOST_FIO_DATA_PATH:${HOST_FIO_DATA_PATH-}
LOCAL_FIO_DATA_PATH=${LOCAL_FIO_DATA_PATH-}
S3_BUCKET_NAME=${S3_BUCKET_NAME-}

--- Other Env Vars ---
GOBIN=${GOBIN-}
GOPATH=${GOPATH-}

---

EOF

if [ -n "${LOCAL_FIO_DATA_PATH-}" ] ; then
    echo "Contents of data dir: '${LOCAL_FIO_DATA_PATH}'"
    ls -oF "${LOCAL_FIO_DATA_PATH}"

    echo "Storage used on: '${LOCAL_FIO_DATA_PATH}'"
    df -h "${LOCAL_FIO_DATA_PATH}"
fi

readonly blinkdisk_exe="${blinkdisk_exe_dir}/blinkdisk"

# Extract git metadata from the exe repo and build blinkdisk
pushd "${blinkdisk_exe_dir}"

readonly blinkdisk_git_revision=$(git rev-parse --short HEAD)
readonly blinkdisk_git_branch="$(git describe --tags --always --dirty)"
readonly blinkdisk_git_dirty=$(git diff-index --quiet HEAD -- || echo "*")
readonly blinkdisk_build_time=$(date +%FT%T%z)

go build -o "${blinkdisk_exe}" github.com/blinkdisk/core

popd

# Extract git metadata on the recovery repo, perform a recovery run
pushd "${blinkdisk_recovery_dir}"

readonly robustness_git_revision=$(git rev-parse --short HEAD)
readonly robustness_git_branch="$(git describe --tags --always --dirty)"
readonly robustness_git_dirty=$(git diff-index --quiet HEAD -- || echo "*")
readonly robustness_build_time=$(date +%FT%T%z)

readonly ld_flags="\
-X github.com/blinkdisk/core/tests/robustness/engine.repoBuildTime=${blinkdisk_build_time} \
-X github.com/blinkdisk/core/tests/robustness/engine.repoGitRevision=${blinkdisk_git_dirty:-""}${blinkdisk_git_revision} \
-X github.com/blinkdisk/core/tests/robustness/engine.repoGitBranch=${blinkdisk_git_branch} \
-X github.com/blinkdisk/core/tests/robustness/engine.testBuildTime=${robustness_build_time} \
-X github.com/blinkdisk/core/tests/robustness/engine.testGitRevision=${robustness_git_dirty:-""}${robustness_git_revision} \
-X github.com/blinkdisk/core/tests/robustness/engine.testGitBranch=${robustness_git_branch}"

readonly test_flags="-v -timeout=${test_timeout}\
 --repo-path-prefix=${test_repo_path_prefix}\
 -ldflags '${ld_flags}'"

ENGINE_MODE="${ENGINE_MODE:-}"
make_target="recovery-tests"

# Run the recovery tests
set -o verbose

make -C "${blinkdisk_recovery_dir}" \
    BLINKDISK_EXE="${blinkdisk_exe}" \
    GO_TEST='go test' \
    TEST_FLAGS="${test_flags}" \
    "${make_target}"

popd
