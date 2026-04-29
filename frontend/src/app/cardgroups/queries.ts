import { graphql } from "@/generated";

// Cardgroup queries

export const MyCardgroupsQuery = graphql(`
  query MyCardgroups {
    myCardgroups {
      id
      name
      updatedAt
    }
  }
`);

export const CardgroupQuery = graphql(`
  query Cardgroup($id: ID!) {
    cardgroup(id: $id) {
      id
      name
      updatedAt
    }
  }
`);

// Cardgroup mutations

export const CreateCardgroupMutation = graphql(`
  mutation CreateCardgroup($input: NewCardgroupInput!) {
    createCardgroup(input: $input) {
      cardgroup {
        id
        name
        updatedAt
      }
    }
  }
`);

export const UpdateCardgroupMutation = graphql(`
  mutation UpdateCardgroup($id: ID!, $input: UpdateCardgroupInput!) {
    updateCardgroup(id: $id, input: $input) {
      cardgroup {
        id
        name
        updatedAt
      }
    }
  }
`);

export const DeleteCardgroupMutation = graphql(`
  mutation DeleteCardgroup($id: ID!) {
    deleteCardgroup(id: $id)
  }
`);

// Card queries

export const CardsByCardgroupQuery = graphql(`
  query CardsByCardgroup($cardgroupId: ID!) {
    cardsByCardgroup(cardgroupId: $cardgroupId) {
      id
      front
      back
      due
      state
      cardgroupId
    }
  }
`);

// Card mutations

export const CreateCardMutation = graphql(`
  mutation CreateCard($input: NewCardInput!) {
    createCard(input: $input) {
      card {
        id
        front
        back
        due
        state
        cardgroupId
      }
    }
  }
`);

export const UpdateCardMutation = graphql(`
  mutation UpdateCard($id: ID!, $input: UpdateCardInput!) {
    updateCard(id: $id, input: $input) {
      card {
        id
        front
        back
        due
        state
        cardgroupId
      }
    }
  }
`);

export const DeleteCardMutation = graphql(`
  mutation DeleteCard($id: ID!) {
    deleteCard(id: $id)
  }
`);
